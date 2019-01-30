package fq

import (
	"errors"
)

func Parse(tokens ...string) (Expr, error) {
	scope := &Scope{
		bracket: false,
		index:   0,
		tokens:  tokens,
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
	bracket bool
	index   int
	tokens  []string
}

func (scope *Scope) ParseWithScope(bracket bool) (Expr, error) {
	subScope := &Scope{
		bracket: bracket,
		index:   0,
		tokens:  scope.tokens[scope.index:],
	}

	subExpr, err := subScope.Parse()
	if err != nil {
		return nil, err
	}

	scope.index += subScope.index

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
			if scope.bracket {
				return nil, errors.New("missing ')'")
			}
			return expr, nil
		}

		switch token {
		case "(":
			subExpr, err := scope.ParseWithScope(true)
			if err != nil {
				return nil, err
			}

			expr = And(expr, subExpr)

		case ")":
			if !scope.bracket {
				return nil, errors.New("invalid character ')'")
			}

			return expr, nil

		case "-a", "-and":
			subExpr, err := scope.ParseWithScope(false)
			if err != nil {
				return nil, err
			}

			expr = And(expr, subExpr)

		case "-o", "-or":
			subExpr, err := scope.ParseWithScope(false)
			if err != nil {
				return nil, err
			}

			expr = Or(expr, subExpr)

		case "!", "-not":
			subExpr, err := scope.ParseWithScope(false)
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
