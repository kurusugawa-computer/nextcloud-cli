package query

import (
	"errors"
)

func Parse(tokens ...string) (Expr, error) {
	scope := &Scope{
		tokens:   tokens,
		index:    0,
		brackets: 0,
	}
	return scope.Parse()
}

type Parser interface {
	Parse(*Scope) (Expr, error)
}

type ParserFunc func(*Scope) (Expr, error)

func (f ParserFunc) Parse(scope *Scope) (Expr, error) {
	return f(scope)
}

type Scope struct {
	tokens   []string
	index    int
	brackets int
}

func (scope *Scope) ParseWithScope() (Expr, error) {
	subScope := &Scope{
		tokens:   scope.tokens[scope.index:],
		index:    0,
		brackets: scope.brackets,
	}

	subExpr, err := subScope.Parse()
	if err != nil {
		return nil, err
	}

	scope.index += subScope.index
	scope.brackets = subScope.brackets

	return subExpr, nil
}

func (scope *Scope) Next() (string, bool) {
	if scope.index >= len(scope.tokens) {
		return "", false
	}
	token := scope.tokens[scope.index]
	scope.index++
	return token, true
}

func (scope *Scope) Parse() (Expr, error) {
	var expr Expr = ExprFunc(ExprNop)

	for {
		token, ok := scope.Next()
		if !ok {
			if scope.brackets > 0 {
				return nil, errors.New("missing ')'")
			}
			return expr, nil
		}

		switch token {
		case "(":
			scope.brackets++

			subExpr, err := scope.ParseWithScope()
			if err != nil {
				return nil, err
			}

			expr = And(expr, subExpr)

		case ")":
			scope.brackets--

			if scope.brackets < 0 {
				return nil, errors.New("invalid character ')'")
			}

			return expr, nil

		case "-a", "-and":
			subExpr, err := scope.ParseWithScope()
			if err != nil {
				return nil, err
			}

			expr = And(expr, subExpr)

		case "-o", "-or":
			subExpr, err := scope.ParseWithScope()
			if err != nil {
				return nil, err
			}

			expr = Or(expr, subExpr)

		case "!", "-not":
			subExpr, err := scope.ParseWithScope()
			if err != nil {
				return nil, err
			}

			expr = And(expr, Not(subExpr))

		default:
			name := token

			parser, ok := Conditions[name]
			if !ok {
				return nil, errors.New("unknown keyword '" + name + "'")
			}

			condExpr, err := parser.Parse(scope)
			if err != nil {
				return nil, err
			}

			expr = And(expr, condExpr)
		}
	}
}
