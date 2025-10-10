package main

import (
	"fmt"

	"github.com/ProjectApario/merkel/tree"
)

func main() {
	data := [][]byte{[]byte("A"), []byte("B"), []byte("C"), []byte("D")}
	mtree, err := tree.Build(data, true) // Parallel build
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	mtree.Print()
	fmt.Println("Root Hash:", mtree.Root.Hash)

	// Append and rebuild concurrently
	err = mtree.Append([]byte("E"), true)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Check membership
	targetHash := tree.CalculateHash([]byte("A"))
	proof, err := mtree.GenerateProof(targetHash)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	isValid := tree.VerifyProof(mtree.Root.Hash, targetHash, proof)
	fmt.Println("Is valid:", isValid)
}
