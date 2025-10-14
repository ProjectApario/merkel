package tree

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Helper function to create test data. Note: It's already sorted alphabetically.
func createTestData(count int) [][]byte {
	var data [][]byte
	for i := 0; i < count; i++ {
		data = append(data, []byte(string(rune('A'+i))))
	}
	return data
}

func TestNewEmptyAndData(t *testing.T) {
	t.Run("NewEmpty", func(t *testing.T) {
		mtree := NewEmpty()
		if mtree == nil {
			t.Fatal("NewEmpty returned nil")
		}
		if mtree.Root != nil {
			t.Error("Expected nil root for empty tree")
		}
		if len(mtree.Data()) != 0 {
			t.Errorf("Expected 0 data elements, got %d", len(mtree.Data()))
		}
	})

	t.Run("Data", func(t *testing.T) {
		data := createTestData(4)
		mtree, _ := Build(data, false)
		if !reflect.DeepEqual(data, mtree.Data()) {
			t.Error("Data() did not return the expected data slice")
		}
	})
}

func TestBuild(t *testing.T) {
	t.Run("ErrorOnNoData", func(t *testing.T) {
		_, err := Build([][]byte{}, false)
		if err == nil {
			t.Fatal("Expected error for empty data, got nil")
		}
		expectedErr := "no data provided"
		if err.Error() != expectedErr {
			t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
		}
	})

	t.Run("SortsUnsortedData", func(t *testing.T) {
		unsorted := [][]byte{[]byte("D"), []byte("A"), []byte("C"), []byte("B")}
		mtree, err := Build(unsorted, false)
		if err != nil {
			t.Fatalf("Build failed: %v", err)
		}
		isSorted := sort.SliceIsSorted(mtree.Data(), func(i, j int) bool {
			return bytes.Compare(mtree.Data()[i], mtree.Data()[j]) < 0
		})
		if !isSorted {
			t.Error("Build function did not sort the initial data")
		}
	})

	testCases := []struct {
		name         string
		dataCount    int
		parallel     bool
		expectedRoot string
	}{
		{"SingleLeaf", 1, false, "559aead08264d5795d3909718cdd05abd49572e84fe55590eef31a88a08fdffd"},
		{"TwoLeaves_Sequential", 2, false, "b30ab174f7459cdd40a3acdf15d0c9444fec2adcfb9d579aa154c084885edd0a"},
		{"TwoLeaves_Parallel", 2, true, "b30ab174f7459cdd40a3acdf15d0c9444fec2adcfb9d579aa154c084885edd0a"},
		{"OddLeaves_Sequential", 3, false, "bd144551df2d311a947adbb0fc4b4bfa7bd76ae0e60095d88a8f79a3e3986af6"},
		{"OddLeaves_Parallel", 3, true, "bd144551df2d311a947adbb0fc4b4bfa7bd76ae0e60095d88a8f79a3e3986af6"},
		{"EvenLeaves_Sequential", 4, false, "50a504831bd50fee3581d287168a85a8dcdd6aa777ffd0fe35e37290268a0153"},
		{"EvenLeaves_Parallel", 4, true, "50a504831bd50fee3581d287168a85a8dcdd6aa777ffd0fe35e37290268a0153"},
		{"LargeOdd_Parallel", 13, true, "6f8a5575b5773729954c4d604dc88227682f1cdefe6ce387a7d9c7fbfc63135b"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := createTestData(tc.dataCount)
			mtree, err := Build(data, tc.parallel)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}
			if mtree.Root.Hash != tc.expectedRoot {
				t.Errorf("Expected root hash %s, got %s", tc.expectedRoot, mtree.Root.Hash)
			}
			if !reflect.DeepEqual(mtree.Data(), data) {
				t.Error("Internal data not stored correctly after build")
			}
		})
	}
}

func TestAppend(t *testing.T) {
	// Start with a non-continuous set to test insertion logic
	data := [][]byte{[]byte("A"), []byte("D"), []byte("F")}
	mtree, _ := Build(data, false)

	t.Run("InsertsInMiddle", func(t *testing.T) {
		err := mtree.Append([]byte("C"), false)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		expectedData := [][]byte{[]byte("A"), []byte("C"), []byte("D"), []byte("F")}
		if !reflect.DeepEqual(mtree.Data(), expectedData) {
			t.Errorf("Append did not insert data in correct sorted order. Got %s", mtree.Data())
		}
	})

	t.Run("InsertsAtStart", func(t *testing.T) {
		err := mtree.Append([]byte("AA"), true) // Test parallel rebuild
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		// --- FIX: Corrected the expected sort order ---
		expectedData := [][]byte{[]byte("A"), []byte("AA"), []byte("C"), []byte("D"), []byte("F")}
		if !reflect.DeepEqual(mtree.Data(), expectedData) {
			t.Errorf("Append did not insert data at start. Got %v", mtree.Data())
		}
	})

	t.Run("ErrorOnDuplicate", func(t *testing.T) {
		err := mtree.Append([]byte("C"), false)
		if err == nil {
			t.Fatal("Expected error when appending a duplicate item, but got nil")
		}
		expectedErr := "username already exists"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})
}

func TestProofAndVerification(t *testing.T) {
	data := createTestData(8) // A-H
	mtree, _ := Build(data, true)

	t.Run("ValidProofs", func(t *testing.T) {
		for _, datum := range data {
			targetHash := CalculateHash(datum)
			proof, err := mtree.GenerateProof(targetHash)
			if err != nil {
				t.Fatalf("Failed to generate proof for %s: %v", string(datum), err)
			}

			isValid := VerifyProof(mtree.Root.Hash, targetHash, proof)
			if !isValid {
				t.Errorf("Proof verification failed for %s", string(datum))
			}
		}
	})

	t.Run("ProofForLeafInRightSubtree", func(t *testing.T) {
		targetHash := CalculateHash([]byte("B"))
		proof, err := mtree.GenerateProof(targetHash)
		if err != nil {
			t.Fatalf("Failed to generate proof for 'B': %v", err)
		}
		if !VerifyProof(mtree.Root.Hash, targetHash, proof) {
			t.Error("Proof verification failed for 'B'")
		}
	})

	t.Run("ProofForSingleNodeTree", func(t *testing.T) {
		sTree, _ := Build(createTestData(1), false)
		targetHash := CalculateHash(sTree.Data()[0])
		proof, err := sTree.GenerateProof(targetHash)
		if err != nil {
			t.Fatalf("GenerateProof for single node failed: %v", err)
		}
		if len(proof.Siblings) != 0 {
			t.Errorf("Expected 0 siblings for single node proof, got %d", len(proof.Siblings))
		}
		if !VerifyProof(sTree.Root.Hash, targetHash, proof) {
			t.Error("Single node proof should be valid")
		}
	})

	t.Run("ErrorOnProofForNonExistentData", func(t *testing.T) {
		_, err := mtree.GenerateProof(CalculateHash([]byte("Z")))
		if err == nil {
			t.Fatal("Expected error for non-existent data proof, got nil")
		}
	})

	t.Run("InvalidProofVerification", func(t *testing.T) {
		targetHash := CalculateHash([]byte("A"))
		proof, _ := mtree.GenerateProof(targetHash)
		proof.Siblings[0] = CalculateHash([]byte("tampered"))
		if VerifyProof(mtree.Root.Hash, targetHash, proof) {
			t.Error("Verification should fail for a tampered proof")
		}
	})
}

func TestNonInclusionProof(t *testing.T) {
	data := [][]byte{[]byte("B"), []byte("D"), []byte("F"), []byte("H")}
	mtree, _ := Build(data, false)
	rootHash := mtree.Root.Hash

	t.Run("ErrorOnExistingItem", func(t *testing.T) {
		_, err := mtree.GenerateNonInclusionProof([]byte("D"))
		if err == nil {
			t.Fatal("Expected error generating non-inclusion proof for existing item")
		}
	})

	t.Run("VerifyItemInMiddle", func(t *testing.T) {
		proof, err := mtree.GenerateNonInclusionProof([]byte("C"))
		if err != nil {
			t.Fatalf("Error generating non-inclusion proof for 'C': %v", err)
		}
		if !VerifyNonInclusionProof(rootHash, []byte("C"), proof) {
			t.Error("Non-inclusion proof for 'C' failed verification")
		}
	})

	t.Run("VerifyItemAtStart", func(t *testing.T) {
		proof, err := mtree.GenerateNonInclusionProof([]byte("A"))
		if err != nil {
			t.Fatalf("Error generating non-inclusion proof for 'A': %v", err)
		}
		if proof.LeftData != nil {
			t.Error("Expected nil left neighbor for item at start")
		}
		if !VerifyNonInclusionProof(rootHash, []byte("A"), proof) {
			t.Error("Non-inclusion proof for 'A' failed verification")
		}
	})

	t.Run("VerifyItemAtEnd", func(t *testing.T) {
		proof, err := mtree.GenerateNonInclusionProof([]byte("Z"))
		if err != nil {
			t.Fatalf("Error generating non-inclusion proof for 'Z': %v", err)
		}
		if proof.RightData != nil {
			t.Error("Expected nil right neighbor for item at end")
		}
		if !VerifyNonInclusionProof(rootHash, []byte("Z"), proof) {
			t.Error("Non-inclusion proof for 'Z' failed verification")
		}
	})

	t.Run("VerifyOnEmptyTree", func(t *testing.T) {
		emptyTree := NewEmpty()
		proof, err := emptyTree.GenerateNonInclusionProof([]byte("A"))
		if err != nil {
			t.Fatalf("Error generating proof for empty tree: %v", err)
		}
		if !VerifyNonInclusionProof("", []byte("A"), proof) {
			t.Error("Non-inclusion proof for empty tree failed")
		}
	})

	t.Run("FailOnInvalidNeighborProof", func(t *testing.T) {
		proof, _ := mtree.GenerateNonInclusionProof([]byte("C"))
		proof.RightProof.Siblings[0] = CalculateHash([]byte("tampered"))
		if VerifyNonInclusionProof(rootHash, []byte("C"), proof) {
			t.Error("Verification should fail with a tampered neighbor proof")
		}
	})
}

func TestFindLeaf(t *testing.T) {
	t.Run("NilNode", func(t *testing.T) {
		_, err := findLeaf(nil, "somehash", nil)
		if err == nil || err.Error() != "node not found" {
			t.Errorf("Expected 'node not found' error for nil node")
		}
	})

	t.Run("LeafNotFoundInValidTree", func(t *testing.T) {
		mtree, _ := Build(createTestData(2), false)
		_, err := findLeaf(mtree.Root, "nonexistenthash", nil)
		if err == nil || err.Error() != "leaf not found" {
			t.Errorf("Expected 'leaf not found' error for nonexistent hash")
		}
	})
}

func TestPrint(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	mtree, _ := Build(createTestData(2), false)
	mtree.Print()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, mtree.Root.Hash) {
		t.Errorf("PrintThis output did not contain root hash")
	}
	printTree(nil, "")
}
