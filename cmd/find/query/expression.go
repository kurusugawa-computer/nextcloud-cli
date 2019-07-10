package query

import "os"

type Expr interface {
	Apply(string, os.FileInfo) (bool, error)
}

type ExprFunc func(string, os.FileInfo) (bool, error)

func (f ExprFunc) Apply(path string, file os.FileInfo) (bool, error) {
	return f(path, file)
}

func ExprNop(string, os.FileInfo) (bool, error) {
	return true, nil
}

func And(exprs ...Expr) Expr {
	return ExprAnd{Exprs: exprs}
}

type ExprAnd struct {
	Exprs []Expr
}

func (e ExprAnd) Apply(path string, file os.FileInfo) (bool, error) {
	for _, expr := range e.Exprs {
		result, err := expr.Apply(path, file)
		if err != nil {
			return false, err
		}

		if !result {
			return false, nil
		}
	}

	return true, nil
}

func Or(exprs ...Expr) Expr {
	return ExprOr{Exprs: exprs}
}

type ExprOr struct {
	Exprs []Expr
}

func (e ExprOr) Apply(path string, file os.FileInfo) (bool, error) {
	for _, expr := range e.Exprs {
		result, err := expr.Apply(path, file)
		if err != nil {
			return false, err
		}

		if result {
			return true, nil
		}
	}

	return false, nil
}

func Not(expr Expr) Expr {
	return ExprNot{Expr: expr}
}

type ExprNot struct {
	Expr Expr
}

func (e ExprNot) Apply(path string, file os.FileInfo) (bool, error) {
	result, err := e.Expr.Apply(path, file)
	if err != nil {
		return false, err
	}

	return !result, nil
}
