package main

import (
	"fmt"

	"github.com/ProjectApario/merkel/tree"
)

func main() {
	// Note: The Build function will automatically sort this data
	data := [][]byte{[]byte("A"), []byte("C"), []byte("D"), []byte("G")}
	mtree, err := tree.Build(data, true) // Parallel build
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("--- Initial Tree ---")
	mtree.Print()
	fmt.Println("Root Hash:", mtree.Root.Hash)
	fmt.Println()

	// --- 1. Test for INCLUSION (existing functionality) ---
	fmt.Println("--- Testing Inclusion Proof ---")
	targetData := []byte("C")
	targetHash := tree.CalculateHash(targetData)
	proof, err := mtree.GenerateProof(targetHash)
	if err != nil {
		fmt.Printf("Error generating inclusion proof for '%s': %v\n", string(targetData), err)
		return
	}
	isValid := tree.VerifyProof(mtree.Root.Hash, targetHash, proof)
	fmt.Printf("Is user '%s' registered? %t\n", string(targetData), isValid)
	fmt.Println()

	// --- 2. Test for NON-INCLUSION (new functionality) ---
	fmt.Println("--- Testing Non-Inclusion Proofs ---")

	// Scenario A: Prove an available username ("B") is NOT in the tree.
	availableUser := []byte("B")
	nonInclusionProof, err := mtree.GenerateNonInclusionProof(availableUser)
	if err != nil {
		fmt.Printf("Error generating non-inclusion proof for '%s': %v\n", string(availableUser), err)
		return
	}
	isAvailable := tree.VerifyNonInclusionProof(mtree.Root.Hash, availableUser, nonInclusionProof)
	fmt.Printf("Is username '%s' available? %t\n", string(availableUser), isAvailable)

	// Scenario B: Try to generate a non-inclusion proof for an existing user ("D").
	// This should correctly return an error.
	existingUser := []byte("D")
	_, err = mtree.GenerateNonInclusionProof(existingUser)
	if err != nil {
		fmt.Printf("Correctly received error for existing user '%s': %v\n", string(existingUser), err)
	} else {
		fmt.Printf("FAIL: Expected an error for existing user '%s' but got none.\n", string(existingUser))
	}
}
