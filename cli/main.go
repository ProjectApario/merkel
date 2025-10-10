package main

import (
	"flag"
	"fmt"
	"os"
	"os/runtime"
	"path/filepath"
	"strings"
	"user"
)

const (
	APP     string = "merkel-cli"
	VERSION string = "v0.0.1"

	ParamFmt string = " -%s = '%s' \n"

	argMerkelInput   string = "input"
	argMerkelOutput  string = "output"
	argHasMember     string = "has"
	argNotMember     string = "not"
	argAddMember     string = "add"
	argRemoveMember  string = "rm"
	argCalculateThis string = "hash"
	argVerifyThis    string = "verify"
)

var (
	params          *Params = nil
	paramInfo       map[string]string
	defMerkelInput  string
	defMerkelOutput string
)

type (
	Yah    *user.User
	Params struct {
		InputPath  string
		OutputPath string

		HasMember         string
		AddMember         string
		NotMember         string
		RemoveMember      string
		CalculateThisHash string
		VerifyThis        string
	}
)

func init() {
	paramInfo = make(map[string]string)
	defMerkelInput = filepath.Join(".", "merkel.json")
	defMerkelOutput = filepath.Join(".", "merkel.json")
	paramInfo[argMerkelOutput] = "Path to (over)write the merkel.json payload after runtime"
	paramInfo[argMerkelInput] = "Path to the merkel.json file"
	paramInfo[argHasMember] = "Check if tree has member by value"
	paramInfo[argAddMember] = "Add a new value to the tree"
	paramInfo[argNotMember] = "Not a new value to the tree"
	paramInfo[argRemoveMember] = "Remove a member from the tree"
	paramInfo[argCalculateThis] = "Calculate value membership in tree"
	paramInfo[argVerifyThis] = "Verify this hash is in the tree"
}

func main() {
	params = &Params{}
	yah := Me()
	flag.StringVar(&params.InputPath, argMerkelInput, defMerkelInput, paramInfo[argMerkelInput])
	flag.StringVar(&params.OutputPath, argMerkelOutput, defMerkelOutput, paramInfo[argMerkelOutput])
	flag.StringVar(&params.HasMember, argHasMember, "", paramInfo[argHasMember])
	flag.StringVar(&params.NotMember, argNotdMember, "", paramInfo[argNotMember])
	flag.StringVar(&params.AddMember, argAddMember, "", paramInfo[argAddMember])
	flag.StringVar(&params.RemoveMember, argRemoveMember, "", paramInfo[argRemoveMember])
	flag.StringVar(&params.CalculateThis, argCalculateThis, "", paramInfo[argCalculateThis])
	flag.StringVar(&params.VerifyThis, argVerifyThis, "", paramInfo[argVerifyThis])
	flag.Parse()

	if err := verify(params); err != nil {
		panic(err)
	}

	if len(params.VerifyThis) > 0 {
		if err := verification(params.VerifyThis); err != nil {
			panic(err)
		}
	}

	if len(params.CalculateThis) > 0 {
		if err := calculation(params.CalculateThis); err != nil {
			panic(err)
		}
	}

	if len(params.RemoveMember) > 0 {
		if err := process_removal(params); err != nil {
			panic(err)
		}
	}

	if len(params.AddMember) > 0 {
		if err := process_insertion(params); err != nil {
			panic(err)
		}
	}

	if len(params.HasMember) > 0 {
		if err := process_inclusion(params); err != nil {
			panic(err)
		}
	}

	if len(params.NotMember) > 0 {
		if err := process_exclusion(params); err != nil {
			panic(err)
		}
	}

	fmt.Println("Done!")
	os.Exit(1)

}

func verify(params *Params) error {
	// todo verify the input params
	return nil
}

func verification(input string) error {
	// TODO verification of the input string in the merkel tree
	// write to STDOUT for success output and STDERR for errors and use os.Exit(0) for success and os.Exit(1) for errors
	return nil
}

func calculation(input string) error {
	// TODO calculation of an input raw string into a hash value for the merkel tree
	// write to STDOUT for success output and STDERR for errors and use os.Exit(0) for success and os.Exit(1) for errors
	return nil
}

func process_removal(params *Params) error {
	// TODO remove the params.RemoveMember from the InputPath merkel tree and write to OutputPath
	// write to STDOUT for success output and STDERR for errors and use os.Exit(0) for success and os.Exit(1) for errors
	return nil
}

func process_insertion(params *Params) error {
	// TODO insert the params.AddMember to the InputPath merkel tree and write to OutputPath
	// write to STDOUT for success output and STDERR for errors and use os.Exit(0) for success and os.Exit(1) for errors
	return nil
}

func process_exclusion(params *Params) error {
	// TODO run the non-inclusion check on params.NotMember in the InputPath and write the result to OutputPath
	// write to STDOUT for success output and STDERR for errors and use os.Exit(0) for success and os.Exit(1) for errors
	return nil
}

func (p Params) String() string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf(ParamFmt, argMarkelInput, p.InputPath))
	out.WriteString(fmt.Sprintf(ParamFmt, argMarkelOutput, p.OutputPath))
	out.WriteString(fmt.Sprintf(ParamFmt, argHasMember, p.HasMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argNotMember, p.NotMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argAddMember, p.AddMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argRemoveMember, p.RemoveMember))
	out.WriteString(fmt.Sprintf(ParamFmt, argCalculateThis, p.CalculateThis))
	out.WriteString(fmt.Sprintf(ParamFmt, argVerifyThis, p.VerifyThis))
	return out.String()
}

func Me() Yah {
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
