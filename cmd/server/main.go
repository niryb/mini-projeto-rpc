package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"

	remotelist "ifpb/remotelist/pkg"
)

const basePath = "lista_dados"

func main() {
	fmt.Println("Servidor iniciando...")

	rl := remotelist.NewRemoteListWithBase(basePath)

	// carregar estado (snapshot + log)
	if err := rl.LoadFromSnapshot(); err != nil {
		fmt.Println("Erro ao carregar snapshot/log:", err)
	}

	// registrar RPC
	server := rpc.NewServer()
	if err := server.RegisterName("RemoteList", rl); err != nil {
		fmt.Println("Erro ao registrar RemoteList:", err)
		return
	}

	// goroutine que cria snapshots periódicos
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Println("[Snapshot] iniciando...")
			if err := rl.CreateSnapshot(); err != nil {
				fmt.Println("[Snapshot] erro ao criar snapshot:", err)
			} else {
				fmt.Println("[Snapshot] snapshot criado com sucesso")
			}
		}
	}()

	// capturar sinais para snapshot final
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\n[Server] sinal recebido: criando snapshot final e encerrando...")
		if err := rl.CreateSnapshot(); err != nil {
			fmt.Println("[Server] erro ao criar snapshot final:", err)
		} else {
			fmt.Println("[Server] snapshot final criado")
		}
		os.Exit(0)
	}()

	// listener
	addr := "localhost:5000"
	l, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("Erro ao iniciar listener:", err)
		return
	}
	defer l.Close()
	fmt.Println("Servidor ouvindo em", addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}
		go server.ServeConn(conn)
	}
}
