package vm

import (
	"fmt"
	"zumbra/code"
	"zumbra/compiler"
	"zumbra/object"
	"zumbra/object/builtins"
)

const StackSize = 2048
const GlobalSize = 65536
const MaxFrames = 1024

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants   []object.Object
	stack       []object.Object
	sp          int
	globals     []object.Object
	frames      []*Frame
	framesIndex int
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFct := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainClosure := &object.Closure{Fn: mainFct}
	mainFrame := NewFrame(mainClosure, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame
	return &VM{
		constants:   bytecode.Constants,
		stack:       make([]object.Object, StackSize),
		sp:          0,
		globals:     make([]object.Object, GlobalSize),
		frames:      frames,
		framesIndex: 1,
	}
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}

	return vm.stack[vm.sp-1]
}

func (vm *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++
		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op = code.Opcode(ins[ip])

		switch op {
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}

		case code.OpAnd:
			right := vm.pop()
			left := vm.pop()
			result := isTruthy(left) && isTruthy(right)
			vm.push(nativeBoolToBooleanObject(result))

		case code.OpOr:
			right := vm.pop()
			left := vm.pop()
			result := isTruthy(left) || isTruthy(right)
			vm.push(nativeBoolToBooleanObject(result))

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpLessThan, code.OpLessThanOrEqual, code.OpGreaterThanOrEqual:
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}

		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}

		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}

		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}

		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}

		case code.OpPop:
			vm.pop()

		case code.OpJump:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

		case code.OpNull:
			err := vm.push(Null)

			if err != nil {
				return err
			}

		case code.OpSetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}

		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements

			err := vm.push(array)
			if err != nil {
				return err
			}

		case code.OpDict:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			dict, err := vm.buildDict(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}

			vm.sp = vm.sp - numElements

			err = vm.push(dict)
			if err != nil {
				return err
			}

		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}

		case code.OpCall:

			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			err := vm.executeCall(int(numArgs))
			if err != nil {
				return err
			}

		case code.OpReturnValue:
			returnValue := vm.pop()

			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err := vm.push(returnValue)
			if err != nil {
				return err
			}

		case code.OpReturn:
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()
			err := vm.push(vm.stack[frame.basePointer+int(localIndex)])
			if err != nil {
				return err
			}

		case code.OpGetBuiltin:
			builtinIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			definition := builtins.Builtins[builtinIndex]

			err := vm.push(definition.Builtin)

			if err != nil {
				return err
			}

		case code.OpClosure:
			constIndex := code.ReadUint16(ins[ip+1:])
			numFree := code.ReadUint8(ins[ip+3:])
			vm.currentFrame().ip += 3

			err := vm.pushClosure(int(constIndex), int(numFree))
			if err != nil {
				return err
			}

		case code.OpGetFree:
			freeIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			currentClosure := vm.currentFrame().cl
			err := vm.push(currentClosure.Free[freeIndex])
			if err != nil {
				return err
			}
		case code.OpCurrentClosure:
			currentClosure := vm.currentFrame().cl
			err := vm.push(currentClosure)
			if err != nil {
				return err
			}

		case code.OpWhile:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			condition := vm.pop()

			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

			vm.currentFrame().ip = pos - 1

		case code.OpGetAttr:
			attrNameObj := vm.pop()
			attrName, ok := attrNameObj.(*object.String)
			if !ok {
				return fmt.Errorf("attribute name must be a string, got %s", attrNameObj.Type())
			}

			obj := vm.pop()

			switch d := obj.(type) {
			case *object.Date:
				switch attrName.Value {
				case "hour":
					vm.push(&object.Integer{Value: int64(d.Hour)})
				case "minute":
					vm.push(&object.Integer{Value: int64(d.Minute)})
				case "day":
					vm.push(&object.Integer{Value: int64(d.Day)})
				case "second":
					vm.push(&object.Integer{Value: int64(d.Second)})
				case "month":
					vm.push(&object.Integer{Value: int64(d.Month)})
				case "year":
					vm.push(&object.Integer{Value: int64(d.Year)})
				case "fullDate":
					vm.push(&object.String{Value: d.FullDate.String()})
				default:
					return fmt.Errorf("unknown attribute %s for Date", attrName.Value)
				}
			default:
				return fmt.Errorf("object type %s has no attributes", obj.Type())
			}

		}

	}

	return nil
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.INTEGER_OBJ && rightType == object.INTEGER_OBJ:
		return vm.executeBinaryIntegerOperation(op, left, right)

	case leftType == object.FLOAT_OBJ && rightType == object.FLOAT_OBJ:
		return vm.executeFloatOperation(op, left, right)

	case leftType == object.INTEGER_OBJ && rightType == object.FLOAT_OBJ:
		return vm.executeIntLeftFloatRight(op, left, right)

	case leftType == object.FLOAT_OBJ && rightType == object.INTEGER_OBJ:
		return vm.executeIntRightFloatLeft(op, left, right)

	case leftType == object.STRING_OBJ && rightType == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)
	}

	return fmt.Errorf("unsupported types for binary operation: %s %s",
		leftType, rightType)

}

func (vm *VM) executeBinaryIntegerOperation(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	var result int64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		result = leftValue / rightValue
	case code.OpMod:
		result = leftValue % rightValue
	default:
		return fmt.Errorf("unknown integer operator: %d", op)
	}

	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) executeFloatOperation(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Float).Value
	rightValue := right.(*object.Float).Value

	var result float64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		result = leftValue / rightValue
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}

	return vm.push(&object.Float{Value: result})
}

func (vm *VM) executeIntLeftFloatRight(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Float).Value

	var result float64

	switch op {
	case code.OpAdd:
		result = float64(leftValue) + rightValue
	case code.OpSub:
		result = float64(leftValue) - rightValue
	case code.OpMul:
		result = float64(leftValue) * rightValue
	case code.OpDiv:
		result = float64(leftValue) / rightValue
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}

	return vm.push(&object.Float{Value: result})
}

func (vm *VM) executeIntRightFloatLeft(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Float).Value
	rightValue := right.(*object.Integer).Value

	var result float64

	switch op {
	case code.OpAdd:
		result = leftValue + float64(rightValue)
	case code.OpSub:
		result = leftValue - float64(rightValue)
	case code.OpMul:
		result = leftValue * float64(rightValue)
	case code.OpDiv:
		result = leftValue / float64(rightValue)
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}

	return vm.push(&object.Float{Value: result})
}

func (vm *VM) executeBinaryStringOperation(op code.Opcode, left, right object.Object) error {
	if op != code.OpAdd {
		return fmt.Errorf("unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
	}

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.FLOAT_OBJ {
		return vm.executeIntLeftFloatRightComparison(op, left, right)
	}

	if left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ {
		return vm.executeFloatComparison(op, left, right)
	}

	if left.Type() == object.FLOAT_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntRightFloatLeftComparison(op, left, right)
	}

	if left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ {
		return vm.executeStringComparison(op, left, right)
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(right == left))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(right != left))
	default:
		return fmt.Errorf("unknown operator: %d (%s %s)", op, left.Type(), right.Type())
	}
}

func (vm *VM) executeIntLeftFloatRightComparison(op code.Opcode, left, right object.Object) error {
	leftValue := float64(left.(*object.Integer).Value)
	rightValue := right.(*object.Float).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue == rightValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue != rightValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpGreaterThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}
}

func (vm *VM) executeIntRightFloatLeftComparison(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Float).Value
	rightValue := float64(right.(*object.Integer).Value)

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue == rightValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue != rightValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpGreaterThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}
}

func (vm *VM) executeFloatComparison(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Float).Value
	rightValue := right.(*object.Float).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue == rightValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue != rightValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpGreaterThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}
}

func (vm *VM) executeStringComparison(op code.Opcode, left, right object.Object) error {
	if op != code.OpEqual {
		return fmt.Errorf("unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(nativeBoolToBooleanObject(leftValue == rightValue))
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue == leftValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue != leftValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpGreaterThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

func (vm *VM) executeBangOperator() error {
	val := vm.pop()

	switch val {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		return vm.push(False)
	}
}

func (vm *VM) executeMinusOperator() error {
	val := vm.pop()

	if val.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported type for negation: %s", val.Type())
	}

	value := val.(*object.Integer).Value
	return vm.push(&object.Integer{Value: -value})
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
}

func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)

	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: elements}
}

func (vm *VM) buildDict(startIndex, endIndex int) (object.Object, error) {
	dictedPairs := make(map[object.DictKey]object.DictPair)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		pair := object.DictPair{Key: key, Value: value}

		dictKey, ok := key.(object.Dictable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		dictedPairs[dictKey.DictKey()] = pair
	}

	return &object.Dict{Pairs: dictedPairs}, nil
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.DICT_OBJ:
		return vm.executeDictIndex(left, index)
	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())
	}
}

func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

func (vm *VM) executeDictIndex(dict, index object.Object) error {
	dictObject := dict.(*object.Dict)

	key, ok := index.(object.Dictable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := dictObject.Pairs[key.DictKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}
func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

func (vm *VM) callClosure(cl *object.Closure, numArgs int) error {
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d", cl.Fn.NumParameters, numArgs)
	}

	frame := NewFrame(cl, vm.sp-numArgs)
	vm.pushFrame(frame)
	vm.sp = frame.basePointer + cl.Fn.NumLocals

	return nil
}

func (vm *VM) executeCall(numArgs int) error {
	callee := vm.stack[vm.sp-1-numArgs]

	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	default:
		return fmt.Errorf("calling non-function and non-built-in object: %s", callee.Type())
	}
}

func (vm *VM) callBuiltin(builtin *object.Builtin, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]

	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs - 1

	if result != nil {
		vm.push(result)
	} else {
		vm.push(Null)
	}

	return nil
}

func (vm *VM) pushClosure(constIndex int, numFree int) error {
	constant := vm.constants[constIndex]
	function, ok := constant.(*object.CompiledFunction)
	if !ok {
		return fmt.Errorf("not a function: %T", constant)
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp = vm.sp - numFree

	closure := &object.Closure{Fn: function, Free: free}
	return vm.push(closure)
}
