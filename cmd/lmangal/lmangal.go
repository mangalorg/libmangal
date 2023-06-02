package main

import (
	"fmt"
	"github.com/metafates/libmangal"
	"os"
)

func main() {
	os.Exit(mainAux())
}

func mainAux() int {
	client := libmangal.NewClient(libmangal.Options{})

	if len(os.Args) != 2 {
		fmt.Println("Usage: lmangal <provider path>")
		return 1
	}

	handle := client.ProviderHandleFromPath(os.Args[1])
	_, err := handle.Provider()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	return 0
}
