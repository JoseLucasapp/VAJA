package vm

import (
	"fmt"
	"testing"
	"vaja/ast"
	"vaja/compiler"
	"vaja/lexer"
	"vaja/object"
	"vaja/parser"
)

func parse(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}

func testIntegerObject(expected int64, actual object.Object) error {
	result, ok := actual.(*object.Integer)

	if !ok {
		return fmt.Errorf("object is not *object.Integer. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. want=%d, got=%d", expected, result.Value)
	}

	return nil
}

type vmTestCase struct {
	input    string
	expected interface{}
}

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.input)
		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err != nil {
			t.Fatalf("vm error: %s", err)
		}

		stackElem := vm.LastPoppedStackElem()

		testExpectedObject(t, tt.expected, stackElem)
	}
}

func testExpectedObject(t *testing.T, expected interface{}, actual object.Object) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		err := testIntegerObject(int64(expected), actual)
		if err != nil {
			t.Errorf("testIntegerObject failed: %s", err)
		}

	case bool:
		err := testBooleanObject(bool(expected), actual)
		if err != nil {
			t.Errorf("testBooleanObject failed: %s", err)
		}

	case *object.Null:
		if actual != Null {
			t.Errorf("object is not *object.Null: %T (%+v)", actual, actual)
		}

	case string:
		err := testStringObject(expected, actual)

		if err != nil {
			t.Errorf("testStringObject failed: %s", err)
		}

	case []int:
		array, ok := actual.(*object.Array)

		if !ok {
			t.Errorf("object not Array: %T (%+v)", actual, actual)
			return
		}

		if len(array.Elements) != len(expected) {
			t.Errorf("wrong num of elements. want=%d, got=%d",
				len(expected), len(array.Elements))
			return
		}

		for i, expectedElem := range expected {
			err := testIntegerObject(int64(expectedElem), array.Elements[i])
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}

	case map[object.DictKey]int64:
		dict, ok := actual.(*object.Dict)
		if !ok {
			t.Errorf("object is not Dict. got=%T (%+v)", actual, actual)
			return
		}

		if len(dict.Pairs) != len(expected) {
			t.Errorf("dict has wrong number of Pairs. want=%d, got=%d",
				len(expected), len(dict.Pairs))
			return
		}

		for expectedKey, expectedValue := range expected {
			pair, ok := dict.Pairs[expectedKey]
			if !ok {
				t.Errorf("no pair for given key in Pairs")
			}

			err := testIntegerObject(expectedValue, pair.Value)
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}

	}
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
		{"-5", -5},
		{"-10", -10},
		{"-50 + 100 + -50", 0},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	runVmTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"(1 > 2) == true", false},
		{"(1 > 2) == false", true},
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
		{"!(if (false) { 5; })", true},
	}
	runVmTests(t, tests)
}

func testBooleanObject(expected bool, actual object.Object) error {
	result, ok := actual.(*object.Boolean)

	if !ok {
		return fmt.Errorf("object is not *object.Boolean. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. want=%t, got=%t", expected, result.Value)
	}

	return nil
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 } ", 20},
		{"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 < 2) { 10 } else { 20 }", 10},
		{"if (1 > 2) { 10 } else { 20 }", 20},
		{"if (1 > 2) { 10 }", Null},
		{"if (false) { 10 }", Null},
		{"if ((if (false) { 10 })) { 10 } else { 20 }", 20},
	}
	runVmTests(t, tests)
}

func TestGlobalVarStatements(t *testing.T) {
	tests := []vmTestCase{
		{"var one << 1; one", 1},
		{"var one << 1; var two << 2; one + two", 3},
		{"var one << 1; var two << one + one; one + two", 3},
	}
	runVmTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		{`"Vaja"`, "Vaja"},
		{`"Va" + "ja"`, "Vaja"},
		{`"Va" + "ja" + "lang"`, "Vajalang"},
	}
	runVmTests(t, tests)
}

func testStringObject(expected string, actual object.Object) error {
	result, ok := actual.(*object.String)

	if !ok {
		return fmt.Errorf("object is not String. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%q, want=%q",
			result.Value, expected)

	}

	return nil
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"[]", []int{}},
		{"[1, 2, 3]", []int{1, 2, 3}},
		{"[1 + 2, 3 * 4, 5 + 6]", []int{3, 12, 11}},
	}
	runVmTests(t, tests)
}

func TestDictLiterals(t *testing.T) {
	tests := []vmTestCase{
		{
			"{}", map[object.DictKey]int64{},
		},
		{
			"{1: 2, 2: 3}",
			map[object.DictKey]int64{
				(&object.Integer{Value: 1}).DictKey(): 2,
				(&object.Integer{Value: 2}).DictKey(): 3,
			},
		},
		{
			"{1 + 1: 2 * 2, 3 + 3: 4 * 4}",
			map[object.DictKey]int64{
				(&object.Integer{Value: 2}).DictKey(): 4,
				(&object.Integer{Value: 6}).DictKey(): 16,
			},
		},
	}
	runVmTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][1]", 2},
		{"[1, 2, 3][0 + 2]", 3},
		{"[[1, 1, 1]][0][0]", 1},
		{"[][0]", Null},
		{"[1, 2, 3][99]", Null},
		{"[1][-1]", Null},
		{"{1: 1, 2: 2}[1]", 1},
		{"{1: 1, 2: 2}[2]", 2},
		{"{1: 1}[0]", Null},
		{"{}[0]", Null},
	}
	runVmTests(t, tests)
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
	var fivePlusTen << fct() { 5 + 10; };
	fivePlusTen();
	`,
			expected: 15,
		},
		{
			input: `
			var one<< fct() { 1; };
			var two << fct() { 2; };
			one() + two()
			`,
			expected: 3,
		},
		{
			input: `
			var a << fct() { 1 };
			var b << fct() { a() + 1 };
			var c << fct() { b() + 1 };
			c();
			`,
			expected: 3,
		},
	}
	runVmTests(t, tests)
}

func TestFunctionsWithReturnStatement(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
		var earlyExit<< fct() { return 99; 100; };
		earlyExit();
		`,
			expected: 99,
		},
		{
			input: `
		var earlyExit << fct() { return 99; return 100; };
		earlyExit();
		`,
			expected: 99,
		},
	}
	runVmTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
	var noReturn << fct() { };
	noReturn();
	`,
			expected: Null,
		},
		{
			input: `
	var noReturn << fct() { };
	var noReturnTwo << fct() { noReturn(); };
	noReturn();
	noReturnTwo();
	`,
			expected: Null,
		},
	}
	runVmTests(t, tests)
}

func TestFirstClassFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
	var returnsOne << fct() { 1; };
	var returnsOneReturner << fct() { returnsOne; };
	returnsOneReturner()();
	`,
			expected: 1,
		},
	}
	runVmTests(t, tests)
}
