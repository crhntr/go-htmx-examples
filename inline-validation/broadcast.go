package main

import (
	"context"
)

// BroadcastServer is copied from
// https://betterprogramming.pub/how-to-broadcast-messages-in-go-using-channels-b68f42bdf32e
type BroadcastServer struct {
	source         <-chan int
	listeners      []chan int
	addListener    chan chan int
	removeListener chan (<-chan int)
}

func (s *BroadcastServer) Subscribe() <-chan int {
	newListener := make(chan int)
	s.addListener <- newListener
	return newListener
}

func (s *BroadcastServer) CancelSubscription(channel <-chan int) {
	s.removeListener <- channel
}

func newBroadcastServer(ctx context.Context, source <-chan int) *BroadcastServer {
	service := &BroadcastServer{
		source:         source,
		listeners:      make([]chan int, 0),
		addListener:    make(chan chan int),
		removeListener: make(chan (<-chan int)),
	}
	go service.serve(ctx)
	return service
}

func (s *BroadcastServer) serve(ctx context.Context) {
	defer func() {
		for _, listener := range s.listeners {
			if listener != nil {
				close(listener)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case newListener := <-s.addListener:
			s.listeners = append(s.listeners, newListener)
		case listenerToRemove := <-s.removeListener:
			for i, ch := range s.listeners {
				if ch == listenerToRemove {
					s.listeners[i] = s.listeners[len(s.listeners)-1]
					s.listeners = s.listeners[:len(s.listeners)-1]
					close(ch)
					break
				}
			}
		case val, ok := <-s.source:
			if !ok {
				return
			}
			for _, listener := range s.listeners {
				if listener != nil {
					select {
					case listener <- val:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}
