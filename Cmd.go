package main

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "sb",
	Short: "Serverbench supervisor",
}

// token

var token = &cobra.Command{
	Use:   "token",
	Short: "read/write token",
}

var tokenSet = &cobra.Command{
	Use:   "set",
	Short: "write token",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			// TODO bounds-of check
			fmt.Println("please, provide a token")
			panic(errors.New("missing token")) // lol
		}
		err := m.UpdateToken(args[0])
		if err != nil {
			fmt.Println("unable to update token")
			panic(err)
		}
		fmt.Println(args[0])
	},
}

var tokenRead = &cobra.Command{
	Use:   "get",
	Short: "read token",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := m.GetToken()
		if err != nil {
			fmt.Println("unable to read token")
			panic(err)
		}
		fmt.Println(*token)
	},
}

// run

var start = &cobra.Command{
	Use:   "start",
	Short: "starts serverbench supervisor",
	Run: func(cmd *cobra.Command, args []string) {
		err := m.Init(0)
		if err != nil {
			fmt.Println("unable to init")
			panic(err)
		}
	},
}

func init() {
	// token
	token.AddCommand(tokenRead)
	token.AddCommand(tokenSet)
	// root
	rootCmd.AddCommand(token)
	rootCmd.AddCommand(start)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
