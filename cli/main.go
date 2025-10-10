package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ProjectApario/merkel/tree"
)

const (
	APP         string = "merkel-cli"
	VERSION     string = "v0.0.1"
	ParamFmt    string = " -%s = '%s' \n"
	argInput    string = "input"
	argOutput   string = "output"
	argHas      string = "has"
	argNot      string = "not"
	argAdd      string = "add"
	argRemove   string = "rm"
	argHash     string = "hash"
	argVerify   string = "verify"
)

var (
	params          *Params
	paramInfo       map[string]string
	defMerkelInput  string
	defMerkelOutput string
)

// Storage is a simple struct for JSON marshaling.
// Data is base64 encoded to safely handle any []byte content.
type Storage struct {
	B64Data []string `json:"data"`
}

type Params struct {
	InputPath         string
	OutputPath        string
	HasMember         string
	AddMember         string
	NotMember         string
	RemoveMember      string
	CalculateThisHash string
	VerifyThis        string
}

func init() {
	paramInfo = make(map[string]string)
	defMerkelInput = filepath.Join(".", "merkel.json")
	defMerkelOutput = filepath.Join(".", "merkel.json")

	paramInfo[argOutput] = "Path to (over)write the merkel.json payload after runtime"
	paramInfo[argInput] = "Path to the merkel.json file"
	paramInfo[argHas] = "Check if tree has member by value"
	paramInfo[argAdd] = "Add a new value to the tree"
	paramInfo[argNot] = "Check if tree does NOT have member by value"
	paramInfo[argRemove] = "Remove a member from the tree"
	paramInfo[argHash] = "Calculate the SHA-256 hash of a raw string value"
	paramInfo[argVerify] = "Verify this pre-computed hash is in the tree"
}

func main() {
	// Use a logger for more informative error messages
	log.SetFlags(0)

	params = &Params{}
	flag.StringVar(&params.InputPath, argInput, defMerkelInput, paramInfo[argInput])
	flag.StringVar(&params.OutputPath, argOutput, defMerkelOutput, paramInfo[argOutput])
	flag.StringVar(&params.HasMember, argHas, "", paramInfo[argHas])
	flag.StringVar(&params.NotMember, argNot, "", paramInfo[argNot])
	flag.StringVar(&params.AddMember, argAdd, "", paramInfo[argAdd])
	flag.StringVar(&params.RemoveMember, argRemove, "", paramInfo[argRemove])
	flag.StringVar(&params.CalculateThisHash, argHash, "", paramInfo[argHash])
	flag.StringVar(&params.VerifyThis, argVerify, "", paramInfo[argVerify])
	flag.Parse()

	if err := verifyParams(params); err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Dispatch commands. Each command function will os.Exit.
	if len(params.CalculateThisHash) > 0 {
		runCalculation(params.CalculateThisHash)
	}

	if len(params.VerifyThis) > 0 {
		runVerification(params)
	}

	if len(params.RemoveMember) > 0 {
		runRemoval(params)
	}

	if len(params.AddMember) > 0 {
		runInsertion(params)
	}

	if len(params.HasMember) > 0 {
		runInclusion(params)
	}

	if len(params.NotMember) > 0 {
		runExclusion(params)
	}

	fmt.Println("No action specified. Use -h for help.")
	os.Exit(0)
}

// verifyParams ensures that only one action flag is provided at a time.
func verifyParams(p *Params) error {
	actionCount := 0
	if p.HasMember != "" {
		actionCount++
	}
	if p.NotMember != "" {
		actionCount++
	}
	if p.AddMember != "" {
		actionCount++
	}
	if p.RemoveMember != "" {
		actionCount++
	}
	if p.VerifyThis != "" {
		actionCount++
	}
	// Note: -hash is allowed to be combined with others.

	if actionCount > 1 {
		return errors.New("please specify only one action flag (-has, -not, -add, -rm, -verify) at a time")
	}
	return nil
}

// runCalculation calculates and prints the hash of a string.
func runCalculation(input string) {
	hash := tree.CalculateHash([]byte(input))
	fmt.Fprintln(os.Stdout, hash)
	os.Exit(0)
}

// runVerification checks if a pre-computed hash exists in the tree.
func runVerification(p *Params) {
	mtree, err := loadTree(p.InputPath)
	if err != nil {
		log.Fatalf("Error loading tree: %v", err)
	}
	_, err = mtree.GenerateProof(p.VerifyThis)
	if err != nil {
		fmt.Fprintln(os.Stdout, "false")
		os.Exit(0)
	}
	fmt.Fprintln(os.Stdout, "true")
	os.Exit(0)
}

// runRemoval removes a member and saves the new tree state.
func runRemoval(p *Params) {
	mtree, err := loadTree(p.InputPath)
	if err != nil {
		log.Fatalf("Error loading tree: %v", err)
	}

	target := []byte(p.RemoveMember)
	var newData [][]byte
	found := false
	for _, item := range mtree.Data() {
		if !bytes.Equal(item, target) {
			newData = append(newData, item)
		} else {
			found = true
		}
	}

	if !found {
		log.Fatalf("Error: member '%s' not found in tree", p.RemoveMember)
	}

	// Rebuild the tree from the filtered data.
	var newTree *tree.MerkleTree
	if len(newData) > 0 {
		newTree, err = tree.Build(newData, runtime.NumCPU() > 1)
		if err != nil {
			log.Fatalf("Error rebuilding tree after removal: %v", err)
		}
	} else {
		// If all items were removed, create a new empty tree.
		newTree = tree.NewEmpty()
	}

	if err := saveTree(p.OutputPath, newTree); err != nil {
		log.Fatalf("Error saving tree after removal: %v", err)
	}

	fmt.Fprintf(os.Stdout, "Successfully removed '%s'.\n", p.RemoveMember)
	if newTree != nil && newTree.Root != nil {
		fmt.Fprintf(os.Stdout, "New Merkle Root: %s\n", newTree.Root.Hash)
	} else {
		fmt.Fprintln(os.Stdout, "Tree is now empty.")
	}

	os.Exit(0)
}

// runInsertion adds a member and saves the new tree state.
func runInsertion(p *Params) {
	mtree, err := loadTree(p.InputPath)
	if err != nil {
		log.Fatalf("Error loading tree: %v", err)
	}

	if err := mtree.Append([]byte(p.AddMember), runtime.NumCPU() > 1); err != nil {
		log.Fatalf("Error adding member: %v", err)
	}

	if err := saveTree(p.OutputPath, mtree); err != nil {
		log.Fatalf("Error saving tree after insertion: %v", err)
	}

	fmt.Fprintf(os.Stdout, "Successfully added '%s'.\n", p.AddMember)
	fmt.Fprintf(os.Stdout, "New Merkle Root: %s\n", mtree.Root.Hash)
	os.Exit(0)
}

// runInclusion checks if a member exists in the tree.
func runInclusion(p *Params) {
	mtree, err := loadTree(p.InputPath)
	if err != nil {
		log.Fatalf("Error loading tree: %v", err)
	}
	targetHash := tree.CalculateHash([]byte(p.HasMember))
	proof, err := mtree.GenerateProof(targetHash)
	if err != nil {
		fmt.Fprintln(os.Stdout, "false")
		os.Exit(0)
	}
	isValid := tree.VerifyProof(mtree.Root.Hash, targetHash, proof)
	fmt.Fprintln(os.Stdout, isValid)
	os.Exit(0)
}

// runExclusion checks if a member does NOT exist in the tree.
func runExclusion(p *Params) {
	mtree, err := loadTree(p.InputPath)
	if err != nil {
		log.Fatalf("Error loading tree: %v", err)
	}
	target := []byte(p.NotMember)
	proof, err := mtree.GenerateNonInclusionProof(target)
	if err != nil {
		// This error means the item *does* exist, so non-inclusion is false.
		fmt.Fprintln(os.Stdout, "false")
		os.Exit(0)
	}

	isAvailable := tree.VerifyNonInclusionProof(mtree.Root.Hash, target, proof)
	fmt.Fprintln(os.Stdout, isAvailable)
	os.Exit(0)
}

// --- Helper Functions ---

// loadTree reads the JSON file and builds a Merkle tree from it.
// If the file does not exist, it returns an empty tree, ready for additions.
func loadTree(path string) (*tree.MerkleTree, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return tree.NewEmpty(), nil
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}

	if len(fileBytes) == 0 {
		return tree.NewEmpty(), nil
	}

	var s Storage
	if err := json.Unmarshal(fileBytes, &s); err != nil {
		return nil, fmt.Errorf("could not parse JSON from %s: %w", path, err)
	}

	var data [][]byte
	for _, b64 := range s.B64Data {
		item, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 data in file: %w", err)
		}
		data = append(data, item)
	}

	if len(data) == 0 {
		return tree.NewEmpty(), nil
	}

	return tree.Build(data, runtime.NumCPU() > 1)
}

// saveTree writes the tree's data back to a JSON file.
func saveTree(path string, mtree *tree.MerkleTree) error {
	if mtree == nil || len(mtree.Data()) == 0 {
		return os.WriteFile(path, []byte(`{"data":[]}`), 0644)
	}

	s := Storage{B64Data: make([]string, len(mtree.Data()))}
	for i, item := range mtree.Data() {
		s.B64Data[i] = base64.StdEncoding.EncodeToString(item)
	}

	jsonData, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal tree data to JSON: %w", err)
	}

	return os.WriteFile(path, jsonData, 0644)
}

// NOTE: These functions are from your skeleton and are not used by the core logic,
// but are kept for completeness.
func (p Params) String() string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf(ParamFmt, argInput, p.InputPath))
	out.WriteString(fmt.Sprintf(ParamFmt, argOutput, p.OutputPath))
	out.WriteString(fmt.Sprintf(ParamFmt, argHas, p.HasMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argNot, p.NotMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argAdd, p.AddMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argRemove, p.RemoveMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argHash, p.CalculateThisHash))
	out.WriteString(fmt.Sprintf(ParamFmt, argVerify, p.VerifyThis))
	return out.String()
}

func Me() *user.User {
	currentUser, err := user.Current()
	if err != nil {
		return &user.User{
			Uid:      "-1",
			Gid:      "-1",
			Username: "nobody",
			Name:     "nobody",
			HomeDir:  "/dev/null",
		}
	}
	return currentUser
}
