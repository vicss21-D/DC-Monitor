package main

// RingBuffer representa um canal que nunca bloqueia (sem backpressure).
// Se encher com a capacidade máxima, o dado mais antigo "cai" da janela para dar espaço ao novo.
// Esta implementação é ideal para streaming de dados em tempo real onde a perda de dados antigos é aceitável.
type RingBuffer struct {
	// ch é o canal buffered que armazena mensagens com capacidade limitada (janela deslizante)
	ch chan interface{}
}

// NewRingBuffer cria a janela com o tamanho (buffer) especificado.
// A capacidade do buffer define quantas mensagens podem ser armazenadas antes de descartar as antigas.
func RingChannel(size int) *RingBuffer {
	return &RingBuffer{
		ch: make(chan interface{}, size),
	}
}

// Push insere um novo pacote. Implementa a lógica da janela deslizante (sliding window).
// Se o buffer estiver cheio (sem leitores consumindo rápido), remove o pacote mais antigo.
// Esta função NUNCA bloqueia o remetente, garantindo desempenho consistente para broadcast.
func (sw *RingBuffer) Push(msg interface{}) {
	for {
		select {
		case sw.ch <- msg:
			// SUCESSO: A janela ainda tem espaço, o pacote entrou no final.
			return
		default:
			// BACKPRESSURE: A janela está cheia.
			// O cliente web está lento e a represa se formou.

			// Retiramos o pacote mais velho da frente (índice 0) e o descartamos
			select {
			case <-sw.ch:
				// Pacote antigo jogado no lixo
			default:
				// Se cair aqui, outra goroutine já esvaziou a vaga, o loop tenta o Push de novo.
			}
		}
	}
}

// Out retorna um canal de leitura para o loop de processamento (o Broadcaster)
// Múltiplos leitores podem consumir mensagens deste canal sem competição
func (sw *RingBuffer) Out() <-chan interface{} {
	return sw.ch
}
