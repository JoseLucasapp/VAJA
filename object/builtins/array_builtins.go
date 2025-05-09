package builtins

import (
	"math"
	"sort"
	"zumbra/object"
)

func RemoveFromArrayBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return NewError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `removeFromArray` must be ARRAY, got %s", args[0].Type())
			}
			if args[1].Type() != object.INTEGER_OBJ {
				return NewError("index argument to `removeFromArray` must be INTEGER, got %s", args[1].Type())
			}

			arr := args[0].(*object.Array)
			index := args[1].(*object.Integer).Value

			if index < 0 || int(index) >= len(arr.Elements) {
				return NewError("index out of bounds: %d", index)
			}

			arr.Elements = append(arr.Elements[:index], arr.Elements[index+1:]...)
			return arr
		},
	}
}

func AddToArrayStartBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return NewError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `addToArrayStart` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)

			arr.Elements = append([]object.Object{args[1]}, arr.Elements...)
			return arr
		},
	}
}

func AddToArrayEndBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return NewError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `addToArrayEnd` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)

			arr.Elements = append(arr.Elements, args[1])
			return arr
		},
	}
}

func MaxBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return NewError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `max` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			if len(arr.Elements) == 0 {
				return nil
			}
			max := arr.Elements[0]
			for _, el := range arr.Elements[1:] {
				if math.Max(float64(max.(*object.Integer).Value), float64(el.(*object.Integer).Value)) == float64(el.(*object.Integer).Value) {
					max = el
				}
			}
			return max
		},
	}
}

func MinBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return NewError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `min` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			if len(arr.Elements) == 0 {
				return nil
			}

			min := arr.Elements[0]
			for _, el := range arr.Elements[1:] {
				if math.Min(float64(min.(*object.Integer).Value), float64(el.(*object.Integer).Value)) == float64(el.(*object.Integer).Value) {
					min = el
				}
			}
			return min
		},
	}
}

func ArrayFirstBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return NewError("wrong number of arguments. got=%d, want=1", len(args))
			}

			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `first` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			if len(arr.Elements) > 0 {
				return arr.Elements[0]
			}

			return nil
		},
	}
}

func ArrayLastBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return NewError("wrong number of arguments. got=%d, want=1", len(args))
			}

			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `last` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				return arr.Elements[length-1]
			}

			return nil
		},
	}
}

func AllButFirstBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return NewError("wrong number of arguments. got=%d, want=1", len(args))
			}

			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `allButFirst` must be ARRAY, got %s", args[0].Type())
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				newElements := make([]object.Object, length-1, length-1)
				copy(newElements, arr.Elements[1:])
				return &object.Array{Elements: newElements}
			}

			return nil
		},
	}
}

func IndexOfBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return NewError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `indexOf` must be ARRAY, got %s", args[0].Type())
			}
			if args[1].Type() != object.INTEGER_OBJ && args[1].Type() != object.STRING_OBJ {
				return NewError("index argument to `indexOf` must be INTEGER, got %s", args[1].Type())
			}

			var index any
			var typeOf string

			arr := args[0].(*object.Array)
			if args[1].Type() == object.INTEGER_OBJ {
				index = args[1].(*object.Integer).Value
				typeOf = object.INTEGER_OBJ
			}

			if args[1].Type() == object.STRING_OBJ {
				index = args[1].(*object.String).Value
				typeOf = object.STRING_OBJ
			}

			for i, el := range arr.Elements {
				if typeOf == object.INTEGER_OBJ {
					if el.(*object.Integer).Value == index.(int64) {
						return NewInteger(int64(i))
					}
				}

				if typeOf == object.STRING_OBJ {
					if el.(*object.String).Value == index.(string) {
						return NewInteger(int64(i))
					}
				}

			}
			return NewInteger(-1)
		},
	}
}

func OrganizeBuiltins() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {

			by := "asc"

			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("first argument to `organize` must be ARRAY, got %s", args[0].Type())
			}

			if len(args) > 1 {
				if args[1].Type() == object.STRING_OBJ {
					by = args[1].(*object.String).Value
				}
			}

			arr := args[0].(*object.Array)
			switch by {
			case "asc":
				sort.Slice(arr.Elements, func(i, j int) bool {
					return arr.Elements[i].(*object.Integer).Value < arr.Elements[j].(*object.Integer).Value
				})
			case "desc":
				sort.Slice(arr.Elements, func(i, j int) bool {
					return arr.Elements[i].(*object.Integer).Value > arr.Elements[j].(*object.Integer).Value
				})
			}

			return arr
		},
	}
}

func SumBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {

			if len(args) != 1 {
				return NewError("wrong number of arguments. got=%d, want=1", len(args))
			}

			if args[0].Type() != object.ARRAY_OBJ {
				return NewError("argument to `sum` must be ARRAY, got %s", args[0].Type())
			}

			var sum float64
			var hasFloat bool = false

			for _, el := range args[0].(*object.Array).Elements {
				if el.Type() != object.INTEGER_OBJ && el.Type() != object.FLOAT_OBJ {
					return NewError("argument to `sum` must be INTEGER or FLOAT, got %s", el.Type())
				}

				if el.Type() == object.FLOAT_OBJ {
					sum += el.(*object.Float).Value
					hasFloat = true
				} else {
					sum += float64(el.(*object.Integer).Value)
				}

			}

			if hasFloat {
				return NewFloat(float64(sum))
			} else {
				return NewInteger(int64(sum))
			}
		},
	}
}
