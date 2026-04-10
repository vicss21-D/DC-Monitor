#  Data Center Telemetry Gateway (Arquitetura Edge Computing)

Este projeto implementa um ecossistema completo de telemetria, roteamento e mitigação de falhas para Data Centers de alta densidade. Utilizando o paradigma de **Distributed Edge Computing**, a arquitetura isola o processamento intensivo (Gateway) da camada de interação física (Sensores, Atuadores e Interface Web).

O sistema foi desenhado para suportar altíssimos volumes de dados de rede, garantindo latência sub-milissegundo no processamento, tolerância a falhas de comunicação e capacidade de intervenção autônoma (Auto-Heal) em cenários de degradação computacional.

---

##  1. Topologia de Rede e Roteamento de Borda (Nginx)

Um dos pilares arquiteturais deste projeto é a abstração da rede. A interface visual e os sensores não conhecem o IP do Servidor Gateway, e o Gateway não se comunica diretamente com o navegador do cliente. Tudo é mediado e unificado pelo **Nginx**, que atua como o roteador definitivo da borda.

A configuração do Nginx neste projeto vai muito além de servir arquivos estáticos. Ele resolve três problemas cruciais de sistemas distribuídos:

* **Injeção de DNS via Docker (`extra_hosts`):** Para evitar o antipadrão de hardcodar IPs no código JavaScript (o que obrigaria a recompilar o Front-end a cada mudança de rede local), o JS faz requisições apenas para o seu próprio domínio (`/api/` e `/ws`). O Nginx captura essas rotas e as envia para um upstream abstrato chamado `backend`. O Docker, através do `docker-compose.yml`, injeta uma regra DNS no Nginx traduzindo a palavra `backend` para o IP físico real da Máquina A.
* **Mitigação de CORS (Cross-Origin Resource Sharing):** Como o Nginx unifica a porta de entrada (Porta 80), o navegador web entende que os arquivos HTML e a API de destino pertencem à mesma origem. Isso elimina completamente os bloqueios de segurança CORS do navegador, permitindo requisições HTTP POST fluídas.
* **WebSocket Connection Upgrade:** O protocolo WebSocket exige um handshake HTTP inicial que deve ser "promovido" a um túnel TCP persistente. O Nginx intercepta a rota `/ws`, injeta os cabeçalhos de camada 7 (`Upgrade: websocket` e `Connection: Upgrade`), e repassa o socket cru (Raw TCP) para o servidor Go, mantendo o túnel aberto (via `proxy_read_timeout`) sem consumir threads pesadas do sistema operacional.

---

##  2. O Motor de Física e Redes (Simulação de Hardware)

Os sensores (`cmd/hardware_node`) não geram números aleatórios simples. Eles implementam um motor termodinâmico e de redes (`internal/node/`) que reage às ordens dos atuadores e simula o comportamento real de um rack de servidores.

* **Termodinâmica (Lei de Newton do Resfriamento):** A temperatura do nó não é arbitrária. Ela é o resultado do embate entre o calor gerado pela CPU (assumindo que 1% da energia elétrica consumida vira calor dissipado) e a capacidade de refrigeração do estado atual do HVAC (Desligado = passivo 1%, Balanceado = 10%, Máximo = 30%). A inércia térmica (Thermal Mass) impede mudanças bruscas, criando curvas de aquecimento realistas.
* **Teoria das Filas (M/M/1) e Latência:** A latência de rede é calculada matematicamente com base na utilização de CPU do nó. Conforme a carga do sistema (`Stress`) se aproxima de 100%, a latência cresce exponencialmente (tendendo a picos de 5000ms), simulando com perfeição o enfileiramento de pacotes na placa de rede quando o processador não consegue esvaziar os buffers.
* **Stress Computacional Híbrido:** A CPU sofre gargalo por dois vetores simultâneos: o Throughput bruto de dados em Gbps (peso de 60%) e a tempestade de interrupções de hardware (podendo chegar a 1.2 milhões de interrupções por segundo), o que força o escalonamento de contexto (Context Switch) e derruba o desempenho.

---

##  3. O Gateway Central em Go (Concorrência e Memory Safety)

O Gateway (`cmd/server`) é uma obra de engenharia de concorrência desenhada para evitar bloqueios de I/O (Input/Output) e prevenir a "inanição" (Starvation) de threads sob alta carga.

### Ingestão UDP e Thread Pool
A comunicação entre os sensores e o Gateway usa **UDP** na porta 9000 para remover o overhead do handshake TCP e o controle de congestionamento. O servidor apenas lê os bytes da interface de rede e os atira imediatamente num Canal (`packetQueue`), libertando a porta.
Um **pool estático de 16 Goroutines (workers/threads)** consome essa fila simultaneamente, desserializando o JSON em paralelo e distribuindo a carga matemática por todos os núcleos reais da CPU do host.

### Memória Atômica (Thread-Safety sem Mutex)
Com milhares de pacotes por segundo sendo avaliados por 16 rotinas concorrentes, o uso de travas tradicionais (`sync.Mutex`) para atualizar a hora do último contato de um sensor criaria um gargalo catastrófico. A solução foi a utilização de primitivos atômicos diretos na CPU:
A atualização do Watchdog (`lastSeen`) e dos cronômetros de falha (`criticalStart`) utiliza `sync/atomic.StoreInt64`. Isso instrui o processador a executar a alteração de memória em nível de hardware (O(1)), garantindo Thread-Safety absoluta sem bloquear as demais rotinas.

### Fluxo de Processamento dos Workers (Goroutines)

O processamento da telemetria bruta recebida via UDP é delegado a um pool estático de 16 Workers (Goroutines). Cada Worker opera de forma assíncrona, executando o seguinte ciclo de vida para cada datagrama consumido da fila (`packetQueue`):

1. **Desserialização e Validação:** Converte os bytes puros para a estrutura Go (`protocol.TelemetryPacket`) via parsing JSON. Datagramas malformados ou com IDs de hardware fora do escopo conhecido (1-8) são descartados instantaneamente na borda.
2. **Atualização de Conectividade (Watchdog Atômico):** Registra o timestamp exato da recepção no vetor global `lastSeen` utilizando operações lock-free (`sync/atomic.StoreInt64`). Isso sinaliza ao servidor que o sensor está vivo sem causar "inanição" ou bloqueios por mutex nas outras threads.
3. **Swap do Quadro de Estado:** Aplica um bloqueio rápido (`sync.Mutex` focado) estritamente para sobrescrever a posição correspondente ao nó na matriz `latestPackets`. Este quadro representa a "fotografia" mais recente do Data Center e serve de base para o loteamento de 1Hz do canal normal.
4. **Roteamento de Prioridade (QoS / Via Prioritária):** O Worker avalia a severidade do pacote (`CurrentState == 2`). Se for detectada uma emergência térmica ou computacional, o pacote fura a fila de loteamento e é injetado diretamente no canal de transmissão prioritária (`broadcastCritical`), alcançando o cliente Web instantaneamente.
5. **Gatilho de Auto-Heal:** Durante eventos críticos, o Worker checa e atualiza o cronômetro atômico de falha (`criticalStart`). Se a divergência persistir por mais de 5 segundos contínuos, o próprio Worker invoca a rotina assíncrona `autoHealNode`, que disparará requisições HTTP automáticas de correção para os atuadores físicos. Se o estado reportado for normal, os cronômetros são zerados.

### Motor de Canais e Backpressure (Ring Buffer)
Uma assimetria clássica em sistemas em tempo real é quando o Servidor (Go) é mais rápido do que a capacidade do Navegador (Web) de renderizar. Isso causa o enfileiramento infinito de pacotes na memória RAM do servidor (Memory Leak).
Para aplicar Backpressure, o sistema usa um **Ring Buffer (Janela Deslizante)** customizado:
Se o navegador ficar lento, a fila de envio no Go enche. Quando isso ocorre, o `select` de prioridade captura o preenchimento, consome ativamente o pacote mais velho da fila (Drop-Oldest) e atira no lixo de forma silenciosa, abrindo espaço para o pacote com o timestamp mais recente. O cliente sempre vê o "agora", e a RAM do servidor nunca transborda.

### Auto-Heal e Exponential Backoff
O servidor cruza os dados do Watchdog a cada segundo. Se uma métrica atômica sinalizar que um nó passou mais de 5 segundos em estado de "Alerta Crítico" sem que o humano tenha operado a interface, o Gateway abandona a visualização passiva e torna-se um agente ativo.
Ele inicia requisições HTTP aos atuadores e emprega algoritmos de Exponential Backoff (tentativas repetidas com intervalos crescentes) para garantir que a ordem de ligar a refrigeração máxima e balanceador de carga transpasse instabilidades temporárias de rede.

---

##  4. Contratos de Comunicação (Payloads)

O ecossistema utiliza protocolos específicos otimizados para cada percurso de rede:

**1. Ingestão de Alta Frequência (Sensor -> Gateway via UDP):**
Enviado a cada milissegundo.

    {
        "node_id": 5,
        "timestamp": 1712683000000,
        "tick": 4500,
        "current_state": 0,
        "temperature": 24.5,
        "stress": 45.0,
        "power_draw": 300.5,
        "latency": 3.2,
        "throughput": 0.55,
        "interrupts": 5012.4,
        "hvac_state": 1,
        "lb_active": false
    }

**2. Visualização Multiplexada (Gateway -> Nginx -> Web via WebSocket):**
Enviado a 1Hz em lotes. Possui o campo "type" ("batch" ou "critical") que instrui o Javascript a renderizar normalmente ou a estourar a animação de alerta no Card do sensor furando a fila de renderização.

    {
        "type": "batch", 
        "payload": [ 
            { "node_id": 1, "temperature": 24.0 /* ... */ } 
        ]
    }

**3. Atuação Direta (Nginx -> Gateway -> Atuador via HTTP):**
Usa HTTP síncrono para garantir o Acknowledge (Status 200 OK) da ação corretiva.

    {
        "type": "hvac",
        "signal": "set_max",
        "target_node": 5,
        "requester": "user"
    }

---

##  5. Padrão Golang-Standards

O repositório foi construído seguindo estritamente o `golang-standards/project-layout`, separando domínio, entrada e implantação:

* **`cmd/`**: O ponto de montagem dos binários. Códigos minimalistas que apenas injetam dependências e iniciam serviços (`cmd/server`, `cmd/hardware_node`, `cmd/actuators/hvac`).
* **`pkg/`**: Contratos públicos seguros para importação. A pasta `protocol/` guarda as Structs compartilhadas para serialização e desserialização de toda a topologia, garantindo que se um campo mudar, o erro seja detectado em tempo de compilação em todas as pontas.
* **`internal/`**: A "caixa preta" corporativa. A lógica termodinâmica de simulação e as fórmulas matemáticas de latência residem aqui, onde o compilador Go proíbe ativamente a importação por agentes externos.
* **`docker/`**: O encapsulamento de infraestrutura. Guarda os manifestos independentes de containerização.

---

## 6. Infraestrutura Docker e Multi-Stage Build

A segurança e a performance de Deploy foram garantidas pelo uso intenso de **Multi-Stage Builds** nos Dockerfiles (`server.Dockerfile`, `hvac.Dockerfile`, etc).

1. O estágio builder usa a imagem completa do compilador Go (`golang:alpine`) para baixar dependências, resolver as bibliotecas internas e compilar o código.
2. Usamos a flag `CGO_ENABLED=0` para forçar a criação de um executável estático, blindado contra bibliotecas de sistema operacional em falta (como glibc).
3. O executável puro é transportado para uma imagem final `alpine:latest` extremamente limpa, sem o compilador Go. 
O resultado é uma pegada de disco ínfima (binários de **menos de 20MB**), perfeitos para serem enviados a appliances físicos limitados na borda da rede.

---

## 7. Guia Definitivo de Implantação

Devido ao desacoplamento físico provido pelos arquivos `.env`, a implantação exige **zero recompilação de código**.

### Pré-requisitos
* Máquinas físicas A (Servidor) e B (Borda) interconectadas via LAN/VLAN.
* Docker Engine e Docker Compose instalados.

### Implantação na Máquina A (O Cérebro / Gateway)
1. Transfira a pasta `edge-server/` para a máquina de alta capacidade computacional.
2. Configure o `.env` interno apontando `ACTUATORS_IP` para o IP físico da Máquina B.
3. Inicie o sistema isolado:
   
        cd edge-server
        docker compose up -d --build

### Implantação na Máquina B (A Interação / Sensores, Atuadores e UI)
1. Transfira a pasta `edge-nodes/` para a máquina instalada fisicamente junto aos técnicos.
2. Configure o `.env` interno apontando `GATEWAY_IP` para o IP físico da Máquina A (para que a telemetria UDP ache o alvo e o Nginx encaminhe as conexões de API).
3. Inicie com o docker compose no diretório raíz específico:
   
        cd edge-nodes
        docker compose up -d --build

### Monitoramento e Logs
* **Interface Gráfica:** Aceda a `http://<IP_DA_MAQUINA_B>` no navegador. A negociação WebSocket WS/WSS ocorrerá automaticamente.
* **Auditoria Local:** O arquivo `logs.csv` é gerado continuamente dentro do contêiner do servidor para processamento assíncrono por ferramentas de Big Data.
* **Logs em Tempo Real (Gateway):** Acompanhe as decisões logadas do algoritmo Auto-Heal:
   
        cd edge-server
        docker compose logs -f backend

## Pontos de Atenção Críticos: Rede e Firewall

Por se tratar de uma arquitetura de *Edge Computing* fisicamente distribuída, a comunicação via rede local (LAN) é o coração do sistema. O tráfego falhará silenciosamente se as portas não estiverem estritamente liberadas.

* **O Bloqueio Silencioso do UDP (Especialmente em Windows):** Sistemas operacionais, em particular o **Windows Defender Firewall**, bloqueiam pacotes UDP de entrada por padrão por questões de segurança. Como a Máquina Gateway (Máquina A) recebe milhares de pacotes por segundo via UDP na porta `9000`, **você deve criar uma Regra de Entrada (Inbound Rule) explícita no Firewall da Máquina A permitindo tráfego UDP na porta 9000**. Caso contrário, o servidor Go rodará perfeitamente, mas os pacotes da borda "baterão na parede" do SO host e nunca chegarão ao contêiner Docker.
* **Liberação TCP:** Certifique-se também de que a porta `80` (HTTP) da Máquina B e a porta `8080` (API/WebSocket) da Máquina A estão permitidas no Firewall para garantir o acesso pelo painel Web.

---

## Automação de Deploy (Uso do Makefile)

Para simplificar a orquestração dos contêineres e evitar a digitação repetitiva de comandos complexos do Docker Compose, o projeto inclui um arquivo **`Makefile`** na raiz. Ele atua como um atalho para desenvolvedores e operadores de infraestrutura.

Para visualizar todos os comandos disponíveis e suas descrições detalhadas, execute no terminal da raiz do projeto:

    make help

Comandos comuns incluem atalhos para subir apenas o servidor (`make up-server`), apenas a borda (`make up-nodes`), ou limpar volumes e imagens órfãs (`make clean-all`).

### Ponto de Atenção para Usuários de Windows
A ferramenta `make` é nativa de ecossistemas Unix (Linux/macOS). Se você estiver utilizando o Windows (CMD ou PowerShell), o terminal retornará o erro *"make não é reconhecido como um comando interno"*. 

Para utilizar o Makefile no Windows, você tem três opções principais:
1. **WSL (Recomendado):** Rodar o projeto através do *Windows Subsystem for Linux*.
2. **Chocolatey/Scoop:** Instalar o pacote nativamente executando `choco install make` (como Administrador).
3. **Git Bash:** Utilizar o terminal do Git Bash (se você instalou o pacote de ferramentas adicionais do MinGW).

*Nota: Se não desejar instalar o `make`, todos os comandos de deploy descritos na seção "Guia Rápido" (usando o comando cru `docker compose`) continuarão funcionando perfeitamente em qualquer sistema operacional.*

---

## Acesso e Visualização da Interface

O Dashboard de monitoramento é servido pelo **Nginx** na Máquina B. Ele centraliza a visualização reativa de todos os nós e permite a intervenção manual imediata.

### Como Acessar
Para visualizar o painel, utilize um navegador em qualquer dispositivo conectado à mesma rede local:

* **Acesso Local (na própria Máquina B):** `http://localhost` ou `<IP_DA_MAQUINA_B>`
* **Acesso Externo (Rede Local):** `http://<IP_DA_MAQUINA_B>` (Exemplo: `http://192.168.1.50`)

### Funcionalidades do Painel
1.  **Dashboard em Tempo Real:** O JavaScript abre automaticamente um túnel **WebSocket** com o Gateway. Os dados são atualizados a 1Hz sem necessidade de atualizar a página.
2.  **Segregação por Severidade:** Os cartões dos nós são movidos automaticamente entre as zonas "Operação Normal" e "Atenção Requerida" (Estado Crítico ou Alto) para priorização visual.
3.  **Controle de Atuadores:** Através de *switches* e botões segmentados, o usuário envia comandos **HTTP POST** para o Gateway, que os roteia para os atuadores físicos (HVAC e Load Balancer).
4.  **Gaveta de Detalhes (Side Drawer):** Ao clicar em um nó, uma gaveta lateral exibe métricas detalhadas (Consumo e Latência) e um gráfico histórico de temperatura e *stress* processado via **Chart.js**.
5.  **Indicador de Conectividade:** Caso o *Watchdog* do servidor detecte que um nó não envia dados há mais de 5 segundos, o cartão assume o estado **OFFLINE** (cinza), indicando falha de hardware ou rede na borda.
