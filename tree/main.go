package tree

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
)

// MerkleNode represents a node in the tree.
type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
}

// MerkleTree holds the root and leaf data for rebuilding.
type MerkleTree struct {
	Root *MerkleNode
	data [][]byte // Stored for append/rebuild
}

// MerkleProof represents the proof: siblings and directions.
type MerkleProof struct {
	Siblings []string
	IsLeft   []bool // true if sibling is left
}

// NonInclusionProof proves that a given piece of data does not exist
// in the tree by providing the proofs of its two closest sorted neighbors.
type NonInclusionProof struct {
	LeftProof  *MerkleProof // Proof of the username to the left
	RightProof *MerkleProof // Proof of the username to the right
	LeftData   []byte       // The actual username data for the left neighbor
	RightData  []byte       // The actual username data for the right neighbor
}

// NewEmpty creates and returns a new, empty MerkleTree.
func NewEmpty() *MerkleTree {
	return &MerkleTree{
		data: make([][]byte, 0),
	}
}

// Data returns a copy of the data elements in the tree.
func (t *MerkleTree) Data() [][]byte {
	return t.data
}

// CalculateHash computes SHA-256 hex string of data.
func CalculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// Build constructs a Merkle tree from data. If parallel=true, uses goroutines for hashing per level.
func Build(data [][]byte, parallel bool) (*MerkleTree, error) {
	if len(data) == 0 {
		return nil, errors.New("no data provided")
	}

	sort.Slice(data, func(i, j int) bool {
		return bytes.Compare(data[i], data[j]) < 0
	})

	// Build leaves
	leaves := make([]*MerkleNode, len(data))
	for i, d := range data {
		leaves[i] = &MerkleNode{Hash: CalculateHash(d)}
	}

	root := buildLevels(leaves, parallel)
	return &MerkleTree{Root: root, data: data}, nil
}

// buildLevels iteratively builds tree levels, optionally in parallel.
func buildLevels(nodes []*MerkleNode, parallel bool) *MerkleNode {
	for len(nodes) > 1 {
		numPairs := (len(nodes) + 1) / 2 // Handle odd number of nodes
		newLevel := make([]*MerkleNode, numPairs)

		if parallel {
			// --- Start of Refactored Parallel Logic ---
			numGoroutines := runtime.NumCPU()
			chunkSize := (numPairs + numGoroutines - 1) / numGoroutines
			var wg sync.WaitGroup

			for g := 0; g < numGoroutines; g++ {
				start := g * chunkSize
				end := start + chunkSize
				if end > numPairs {
					end = numPairs
				}

				// The core fix for the panic: don't start a goroutine with no work.
				if start >= end {
					continue
				}

				wg.Add(1)
				go func(offset, start, end int) {
					defer wg.Done()
					for i := start; i < end; i++ {
						leftIdx := i * 2
						rightIdx := leftIdx + 1

						left := nodes[leftIdx]
						var right *MerkleNode
						if rightIdx < len(nodes) {
							right = nodes[rightIdx]
						} else {
							right = left // Duplicate the left node if it's the last one
						}

						parentHash := CalculateHash([]byte(left.Hash + right.Hash))
						parent := &MerkleNode{Hash: parentHash, Left: left, Right: right}
						newLevel[offset+i] = parent // Place result in the correct, pre-allocated slot
					}
				}(0, start, end) // Passing 0 for offset as we map directly to newLevel indices
			}
			wg.Wait()
			// --- End of Refactored Parallel Logic ---
		} else {
			// Sequential logic is fine, let's keep it.
			for i := 0; i < numPairs; i++ {
				leftIdx := i * 2
				rightIdx := leftIdx + 1

				left := nodes[leftIdx]
				var right *MerkleNode
				if rightIdx < len(nodes) {
					right = nodes[rightIdx]
				} else {
					right = left // Duplicate the left node if it's the last one
				}

				parentHash := CalculateHash([]byte(left.Hash + right.Hash))
				newLevel[i] = &MerkleNode{Hash: parentHash, Left: left, Right: right}
			}
		}
		nodes = newLevel
	}
	return nodes[0]
}

func (t *MerkleTree) Append(newData []byte, parallel bool) error {
	// Find the correct index to insert the new data to maintain sort order
	index := sort.Search(len(t.data), func(i int) bool {
		// Find the first element that is >= newData
		return bytes.Compare(t.data[i], newData) >= 0
	})

	// Check if the username already exists at that index
	if index < len(t.data) && bytes.Equal(t.data[index], newData) {
		return errors.New("username already exists")
	}

	// Insert the new data at the correct index to maintain the sorted slice
	t.data = append(t.data[:index], append([][]byte{newData}, t.data[index:]...)...)

	// Rebuild the tree with the new, sorted data
	newRoot := buildLevels(t.leaves(), parallel)
	t.Root = newRoot
	return nil
}

// leaves rebuilds leaf slice (internal).
func (t *MerkleTree) leaves() []*MerkleNode {
	leaves := make([]*MerkleNode, len(t.data))
	for i, d := range t.data {
		leaves[i] = &MerkleNode{Hash: CalculateHash(d)}
	}
	return leaves
}

// GenerateProof creates a proof for the target hash.
func (t *MerkleTree) GenerateProof(targetHash string) (*MerkleProof, error) {
	path, err := findLeaf(t.Root, targetHash, nil)
	if err != nil {
		return nil, err
	}

	if len(path) == 0 { // This happens if the tree has only one node (the root is the leaf)
		return &MerkleProof{}, nil
	}

	var siblings []string
	var isLeft []bool

	// --- Start of Fix ---
	// 1. Preserve the leaf's immediate parent before the loop.
	leafParent := path[len(path)-1]

	// 2. Use a separate variable, `traversalNode`, to walk up the tree.
	traversalNode := leafParent
	for i := len(path) - 2; i >= 0; i-- {
		parent := path[i]
		if parent.Left == traversalNode {
			// Sibling is on the right
			siblings = append(siblings, parent.Right.Hash)
			isLeft = append(isLeft, false)
		} else {
			// Sibling is on the left
			siblings = append(siblings, parent.Left.Hash)
			isLeft = append(isLeft, true)
		}
		traversalNode = parent // Continue traversal up
	}

	// 3. Now, use the preserved `leafParent` to correctly find the leaf's sibling.
	if leafParent.Left != nil && leafParent.Left.Hash == targetHash {
		// The target leaf is on the left, its sibling is on the right.
		siblings = append([]string{leafParent.Right.Hash}, siblings...)
		isLeft = append([]bool{false}, isLeft...)
	} else {
		// The target leaf is on the right, its sibling is on the left.
		siblings = append([]string{leafParent.Left.Hash}, siblings...)
		isLeft = append([]bool{true}, isLeft...)
	}
	// --- End of Fix ---

	return &MerkleProof{Siblings: siblings, IsLeft: isLeft}, nil
}

// findLeaf recursively finds path to leaf.
// Small correction: Passing nil for initial path is cleaner.
func findLeaf(node *MerkleNode, targetHash string, path []*MerkleNode) ([]*MerkleNode, error) {
	if node == nil {
		return nil, errors.New("node not found")
	}

	// Found the leaf. The path contains its ancestors.
	if node.Left == nil && node.Right == nil {
		if node.Hash == targetHash {
			return path, nil
		}
		return nil, errors.New("leaf not found")
	}

	// Append current node to the path for the next recursive call
	newPath := append(path, node)

	if node.Left != nil {
		if foundPath, _ := findLeaf(node.Left, targetHash, newPath); foundPath != nil {
			return foundPath, nil
		}
	}

	if node.Right != nil {
		if foundPath, _ := findLeaf(node.Right, targetHash, newPath); foundPath != nil {
			return foundPath, nil
		}
	}

	return nil, errors.New("leaf not found")
}

// VerifyProof checks if proof matches root.
func VerifyProof(rootHash, leafHash string, proof *MerkleProof) bool {
	currentHash := leafHash
	for i := 0; i < len(proof.Siblings); i++ {
		sibling := proof.Siblings[i]
		if proof.IsLeft[i] {
			currentHash = CalculateHash([]byte(sibling + currentHash))
		} else {
			currentHash = CalculateHash([]byte(currentHash + sibling))
		}
	}
	return currentHash == rootHash
}

// Print prints the tree recursively. For large trees, this may cause stack overflow; consider iterative BFS instead.
func (t *MerkleTree) Print() {
	printTree(t.Root, "")
}

func printTree(node *MerkleNode, indent string) {
	if node != nil {
		fmt.Println(indent + "Hash: " + node.Hash)
		if node.Left != nil {
			printTree(node.Left, indent+"  ")
		}
		if node.Right != nil {
			printTree(node.Right, indent+"  ")
		}
	}
}

// GenerateNonInclusionProof creates a proof that the given username does not exist in the tree.
// It does this by finding the username's would-be neighbors in the sorted list of data
// and generating standard inclusion proofs for them.
func (t *MerkleTree) GenerateNonInclusionProof(username []byte) (*NonInclusionProof, error) {
	// Find the index where the username would be inserted to maintain order.
	index := sort.Search(len(t.data), func(i int) bool {
		return bytes.Compare(t.data[i], username) >= 0
	})

	// If the item exists, we cannot generate a non-inclusion proof.
	if index < len(t.data) && bytes.Equal(t.data[index], username) {
		return nil, errors.New("cannot generate non-inclusion proof for an existing item")
	}

	// Handle the case of an empty tree.
	if len(t.data) == 0 {
		return &NonInclusionProof{}, nil
	}

	proof := &NonInclusionProof{}
	var err error

	// A left neighbor exists if the insertion index is not 0.
	if index > 0 {
		proof.LeftData = t.data[index-1]
		leftHash := CalculateHash(proof.LeftData)
		proof.LeftProof, err = t.GenerateProof(leftHash)
		if err != nil {
			return nil, fmt.Errorf("failed to generate proof for left neighbor: %w", err)
		}
	}

	// A right neighbor exists if the insertion index is not at the end of the slice.
	if index < len(t.data) {
		proof.RightData = t.data[index]
		rightHash := CalculateHash(proof.RightData)
		proof.RightProof, err = t.GenerateProof(rightHash)
		if err != nil {
			return nil, fmt.Errorf("failed to generate proof for right neighbor: %w", err)
		}
	}

	return proof, nil
}

// VerifyNonInclusionProof verifies that a username does not exist in a tree with a given root hash.
// It checks the validity of the neighboring proofs and confirms the username sorts between them.
func VerifyNonInclusionProof(rootHash string, username []byte, proof *NonInclusionProof) bool {
	// Case 1: The proof is for an empty tree.
	if proof.LeftData == nil && proof.RightData == nil {
		return true // Correctly proves non-inclusion in an empty set.
	}

	// Case 2: The username should be the first item in the list.
	if proof.LeftData == nil {
		rightHash := CalculateHash(proof.RightData)
		if !VerifyProof(rootHash, rightHash, proof.RightProof) {
			return false // The right neighbor's proof is invalid.
		}
		// Check if the username correctly sorts before its right neighbor.
		return bytes.Compare(username, proof.RightData) < 0
	}

	// Case 3: The username should be the last item in the list.
	if proof.RightData == nil {
		leftHash := CalculateHash(proof.LeftData)
		if !VerifyProof(rootHash, leftHash, proof.LeftProof) {
			return false // The left neighbor's proof is invalid.
		}
		// Check if the username correctly sorts after its left neighbor.
		return bytes.Compare(proof.LeftData, username) < 0
	}

	// Case 4: The username is between two existing items.
	leftHash := CalculateHash(proof.LeftData)
	if !VerifyProof(rootHash, leftHash, proof.LeftProof) {
		return false // Left proof is invalid.
	}

	rightHash := CalculateHash(proof.RightData)
	if !VerifyProof(rootHash, rightHash, proof.RightProof) {
		return false // Right proof is invalid.
	}

	// Check if the username correctly sorts between its neighbors.
	return bytes.Compare(proof.LeftData, username) < 0 && bytes.Compare(username, proof.RightData) < 0
}
