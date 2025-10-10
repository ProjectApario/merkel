package tree

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
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

// Append adds new data and rebuilds the tree (concurrently if parallel=true).
func (t *MerkleTree) Append(newData []byte, parallel bool) error {
	t.data = append(t.data, newData)
	newRoot := buildLevels(t.leaves(), parallel) // Rebuild from updated leaves
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
