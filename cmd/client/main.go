package main

import (
	"bufio"
	"fmt"
	"net/rpc"
	"os"
	"strconv"
	"strings"

	remotelist "ifpb/remotelist/pkg"
)

func readLine(prompt string) string {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func main() {
	fmt.Println("Conectando ao servidor RPC...")
	client, err := rpc.Dial("tcp", "localhost:5000")
	if err != nil {
		fmt.Println("Erro ao conectar:", err)
		return
	}
	defer client.Close()
	fmt.Println("Conectado com sucesso!")

	for {
		fmt.Println("\n========== MENU ==========")
		fmt.Println("1 - Selecionar/usar lista (informar list_id e operações)")
		fmt.Println("2 - Criar/Inicializar lista (opcional)")
		fmt.Println("3 - Ver todas as listas (debug)")
		fmt.Println("4 - Sair")
		opt := readLine("Escolha uma opção: ")

		switch opt {
		case "1":
			idStr := readLine("Digite list_id (inteiro): ")
			listID, err := strconv.Atoi(idStr)
			if err != nil {
				fmt.Println("list_id inválido")
				continue
			}
			operateOnList(client, listID)

		case "2":
			idStr := readLine("Digite list_id a criar/inicializar (inteiro): ")
			listID, err := strconv.Atoi(idStr)
			if err != nil {
				fmt.Println("list_id inválido")
				continue
			}
			var ok remotelist.AppendReply
			args := remotelist.AppendArgs{ListID: listID, Value: 0}
			if err := client.Call("RemoteList.Append", args, &ok); err != nil {
				fmt.Println("Erro ao inicializar lista:", err)
			} else {
				var rem remotelist.RemoveReply
				_ = client.Call("RemoteList.Remove", remotelist.RemoveArgs{ListID: listID}, &rem)
				fmt.Println("Lista inicializada (se não existia).")
			}

		case "3":
			var all map[int][]int
			if err := client.Call("RemoteList.GetLists", struct{}{}, &all); err != nil {
				fmt.Println("Erro ao obter listas:", err)
			} else {
				fmt.Println("Listas no servidor:")
				for k, v := range all {
					fmt.Printf("list_id=%d -> %v\n", k, v)
				}
			}

		case "4":
			fmt.Println("Encerrando cliente...")
			return
		default:
			fmt.Println("Opção inválida")
		}
	}
}

func operateOnList(client *rpc.Client, listID int) {
	for {
		fmt.Printf("\n---- Operando lista %d ----\n", listID)
		fmt.Println("1 - Append (adicionar valor)")
		fmt.Println("2 - Get (pegar posição i)")
		fmt.Println("3 - Remove (remover último)")
		fmt.Println("4 - Size")
		fmt.Println("5 - Voltar")
		choice := readLine("Escolha: ")
		switch choice {
		case "1":
			valStr := readLine("Valor a adicionar (inteiro): ")
			v, err := strconv.Atoi(valStr)
			if err != nil {
				fmt.Println("valor inválido")
				continue
			}
			args := remotelist.AppendArgs{ListID: listID, Value: v}
			var rep remotelist.AppendReply
			if err := client.Call("RemoteList.Append", args, &rep); err != nil {
				fmt.Println("Erro ao adicionar:", err)
			} else {
				fmt.Println("Adicionado com sucesso.")
			}
		case "2":
			idxStr := readLine("Índice (inteiro): ")
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				fmt.Println("índice inválido")
				continue
			}
			args := remotelist.GetArgs{ListID: listID, Index: idx}
			var rep remotelist.GetReply
			if err := client.Call("RemoteList.Get", args, &rep); err != nil {
				fmt.Println("Erro ao obter:", err)
			} else {
				fmt.Printf("Valor em [%d] = %d\n", idx, rep.Value)
			}
		case "3":
			args := remotelist.RemoveArgs{ListID: listID}
			var rep remotelist.RemoveReply
			if err := client.Call("RemoteList.Remove", args, &rep); err != nil {
				fmt.Println("Erro ao remover:", err)
			} else {
				fmt.Printf("Removido: %d\n", rep.Value)
			}
		case "4":
			args := remotelist.SizeArgs{ListID: listID}
			var rep remotelist.SizeReply
			if err := client.Call("RemoteList.Size", args, &rep); err != nil {
				fmt.Println("Erro ao obter tamanho:", err)
			} else {
				fmt.Printf("Tamanho: %d\n", rep.Size)
			}
		case "5":
			return
		default:
			fmt.Println("Opção inválida")
		}
	}
}
