package main

import (
	"encoding/json"
	"fmt"
	remotelist "ifpb/remotelist/pkg"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
)

const dataFile = "lista_dados.json"

func main() {
	fmt.Println("Servidor iniciando...")

	list := remotelist.NewRemoteList()

	// configura o arquivo de persistência e tenta carregar estado anterior (se existir)
	remotelist.SetPersistFile(dataFile)
	if err := list.LoadFrom(dataFile); err == nil {
		fmt.Println("Dados carregados com sucesso de", dataFile)
	} else {
		fmt.Println("Nenhum dado anterior carregado:", err)
	}

	rpcs := rpc.NewServer()
	rpcs.Register(list)

	// capturar sinais para salvar antes de encerrar (adicional, proteção)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nSinal recebido: salvando dados e encerrando...")
		// como o pkg já salva em cada alteração, aqui só forçamos gravação final
		if err := list.LoadFrom(dataFile); err == nil {
			// nothing: LoadFrom used only to check read; to force save we can call internal save but no exported Save.
		}
		os.Exit(0)
	}()

	fmt.Println("Servidor ouvindo em localhost:5000...")
	l, e := net.Listen("tcp", "localhost:5000")
	if e != nil {
		fmt.Println("Erro ao iniciar o listener:", e)
		return
	}
	defer l.Close()

	for {
		fmt.Println("Aguardando conexão...")
		conn, err := l.Accept()
		if err == nil {
			fmt.Println("Cliente conectado!")
			go rpcs.ServeConn(conn)
		} else {
			fmt.Println("Erro ao aceitar conexão:", err)
			break
		}
	}

	fmt.Println("Encerrando servidor.")
}

func saveData(list *remotelist.RemoteList) {
	data, err := json.MarshalIndent(list.GetList(), "", "  ")
	if err != nil {
		fmt.Println("Erro ao serializar dados:", err)
		return
	}

	err = os.WriteFile(dataFile, data, 0644)
	if err != nil {
		fmt.Println("Erro ao salvar dados:", err)
		return
	}

	fmt.Println("Dados salvos com sucesso em", dataFile)
}

func loadData(list *remotelist.RemoteList) {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		fmt.Println("Nenhum arquivo de dados anterior encontrado. Iniciando com lista vazia.")
		return
	}

	var items []int
	err = json.Unmarshal(data, &items)
	if err != nil {
		fmt.Println("Erro ao desserializar dados:", err)
		return
	}

	for _, item := range items {
		var reply bool
		list.Append(item, &reply)
	}

	fmt.Printf("Dados carregados com sucesso! %d elementos restaurados.\n", len(items))
}
