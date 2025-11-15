package main

import (
	"fmt"
	"net/rpc"
)

func main() {
	fmt.Println("Conectando ao servidor...")
	client, err := rpc.Dial("tcp", "localhost:5000")
	if err != nil {
		fmt.Println("Erro ao conectar:", err)
		return
	}
	fmt.Println("Conectado com sucesso!")
	defer client.Close()

	var quantidade int
	fmt.Print("Quantos números você deseja inserir? ")
	fmt.Scan(&quantidade)

	for i := 0; i < quantidade; i++ {
		var num int
		fmt.Printf("Digite o número %d: ", i+1)
		fmt.Scan(&num)

		var reply bool
		err = client.Call("RemoteList.Append", num, &reply)
		if err != nil {
			fmt.Println("Erro ao adicionar:", err)
		} else {
			fmt.Println("Número adicionado com sucesso.")
		}
	}

	// Menu principal
	for {
		fmt.Println("\n========== MENU ==========")
		fmt.Println("1 - Ver tamanho da lista")
		fmt.Println("2 - Exibir elementos da lista")
		fmt.Println("3 - Adicionar números")
		fmt.Println("4 - Remover elemento")
		fmt.Println("5 - Sair")
		fmt.Print("Escolha uma opção: ")

		var opcao int
		fmt.Scan(&opcao)

		switch opcao {
		case 1:
			var size int
			err = client.Call("RemoteList.Size", 0, &size)
			if err != nil {
				fmt.Println("Erro ao obter tamanho:", err)
			} else {
				fmt.Printf("Tamanho atual da lista: %d\n", size)
			}

		case 2:
			var size int
			err = client.Call("RemoteList.Size", 0, &size)
			if err != nil {
				fmt.Println("Erro ao obter tamanho:", err)
			} else if size > 0 {
				fmt.Println("\nElementos da lista:")
				for i := 0; i < size; i++ {
					var element int
					err = client.Call("RemoteList.Get", i, &element)
					if err != nil {
						fmt.Printf("Erro ao obter elemento %d: %v\n", i, err)
					} else {
						fmt.Printf("[%d] = %d\n", i, element)
					}
				}
			} else {
				fmt.Println("Lista vazia!")
			}

		case 3:
			var qtd int
			fmt.Print("Quantos números deseja adicionar? ")
			fmt.Scan(&qtd)

			for i := 0; i < qtd; i++ {
				var num int
				fmt.Printf("Digite o número: ")
				fmt.Scan(&num)

				var reply bool
				err = client.Call("RemoteList.Append", num, &reply)
				if err != nil {
					fmt.Println("Erro ao adicionar:", err)
				} else {
					fmt.Println("Número adicionado com sucesso.")
				}
			}

		case 4:
			var size int
			err = client.Call("RemoteList.Size", 0, &size)
			if err != nil {
				fmt.Println("Erro ao obter tamanho:", err)
			} else if size == 0 {
				fmt.Println("Lista vazia! Nada para remover.")
			} else {
				fmt.Println("\nElementos da lista:")
				for i := 0; i < size; i++ {
					var element int
					err = client.Call("RemoteList.Get", i, &element)
					if err != nil {
						fmt.Printf("Erro ao obter elemento %d: %v\n", i, err)
					} else {
						fmt.Printf("[%d] = %d\n", i, element)
					}
				}

				fmt.Print("\nDeseja remover o último elemento? (s/n): ")
				var confirma string
				fmt.Scan(&confirma)

				if confirma == "s" || confirma == "S" {
					var reply_i int
					err = client.Call("RemoteList.Remove", 0, &reply_i)
					if err != nil {
						fmt.Println("Erro ao remover:", err)
					} else {
						fmt.Printf("Elemento removido: %d\n", reply_i)
					}
				} else {
					fmt.Println("Operação cancelada.")
				}
			}

		case 5:
			fmt.Println("Encerrando...")
			return

		default:
			fmt.Println("Opção inválida!")
		}
	}
}
