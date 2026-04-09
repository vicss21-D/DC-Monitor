package main

// RingBuffer representa um canal que nunca bloqueia.
// Se encher, o dado mais antigo "cai" da janela para dar espaço ao novo.
type RingBuffer struct {
	ch chan interface{}
}

// NewRingBuffer cria a janela com o tamanho (buffer) especificado.
func RingChannel(size int) *RingBuffer {
	return &RingBuffer{
		ch: make(chan interface{}, size),
	}
}

// Push insere um novo pacote. Implementa a lógica da janela deslizante.
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

// Out retorna o canal para o loop de leitura (o Broadcaster)
func (sw *RingBuffer) Out() <-chan interface{} {
	return sw.ch
}