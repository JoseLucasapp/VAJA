package repl

import (
	"bufio"
	"fmt"
	"io"

	"vaja/evaluator"
	"vaja/lexer"
	"vaja/object"
	"vaja/parser"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

	for {
		fmt.Printf(PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		evaluated := evaluator.Eval(program, env)
		if evaluated != nil {
			io.WriteString(out, evaluated.Inspect())
			io.WriteString(out, "\n")
		}
	}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, beer)
	io.WriteString(out, "Woops! We ran into some 'I need a beer' business here!\n")
	io.WriteString(out, "Parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

const beer = `
█▄▀▄▀▄█
█░▀░▀░█▄
█░▀░░░█─█
█░░░▀░█▄▀
▀▀▀▀▀▀▀
`
