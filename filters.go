package rem

import (
	"fmt"
	"strings"
)

var filterOperators = map[string]struct{}{
	"=":          {},
	"!=":         {},
	"<>":         {},
	"<":          {},
	">":          {},
	"<=":         {},
	">=":         {},
	"LIKE":       {},
	"NOT LIKE":   {},
	"IN":         {},
	"NOT IN":     {},
	"IS":         {},
	"IS NOT":     {},
	"ALL":        {},
	"<> ALL":     {},
	"ANY":        {},
	"<> ANY":     {},
	"EXISTS":     {},
	"NOT EXISTS": {},
	"OVERLAPS":   {},
	"?":          {},
	"?&":         {},
	"?|":         {},
	"@>":         {},
	"<@":         {},
}

type FilterClause struct {
	Left     interface{}
	Operator string
	Right    interface{}
	Rule     string
}

func (filter FilterClause) leftString(dialect Dialect, args []interface{}) ([]interface{}, string, error) {
	switch left := filter.Left.(type) {
	case string:
		return args, dialect.QuoteIdentifier(left), nil

	case DialectStringerWithArgs:
		lv, args, err := left.StringWithArgs(dialect, args)
		return args, lv, err

	case DialectStringer:
		return args, left.StringForDialect(dialect), nil

	case SqlUnsafe:
		return args, left.Sql, nil
	}

	return nil, "", fmt.Errorf("rem: unsupported type for left side of filter clause '%#v'", filter.Left)
}

func (filter FilterClause) rightString(dialect Dialect, args []interface{}) ([]interface{}, string, error) {
	switch right := filter.Right.(type) {
	case DialectStringerWithArgs:
		rv, args, err := right.StringWithArgs(dialect, args)
		return args, rv, err

	case DialectStringer:
		return args, right.StringForDialect(dialect), nil

	case SqlUnsafe:
		return args, right.Sql, nil

	case nil:
		return args, "NULL", nil

	case []interface{}:
		var sliceArgs strings.Builder
		for j, arg := range right {
			args = append(args, arg)
			if j > 0 {
				sliceArgs.WriteString(",")
			}
			sliceArgs.WriteString(dialect.Param(len(args)))
		}
		return args, sliceArgs.String(), nil

	default:
		args = append(args, right)
		return args, dialect.Param(len(args)), nil
	}
}

func (filter FilterClause) StringWithArgs(dialect Dialect, args []interface{}) (string, []interface{}, error) {
	switch filter.Rule {
	case "(":
		return " (", args, nil

	case ")":
		return " )", args, nil

	case "AND":
		return " AND", args, nil

	case "OR":
		return " OR", args, nil

	case "WHERE":
		if _, ok := filterOperators[filter.Operator]; !ok {
			return "", nil, fmt.Errorf("rem: invalid operator '%s' on WHERE clause", filter.Operator)
		}

		var err error
		var left string
		args, left, err = filter.leftString(dialect, args)
		if err != nil {
			return "", nil, err
		}

		var right string
		args, right, err = filter.rightString(dialect, args)
		if err != nil {
			return "", nil, err
		}

		if filter.Operator == "EXISTS" || filter.Operator == "NOT EXISTS" {
			return fmt.Sprintf(" %s (%s)", filter.Operator, right), args, nil
		} else if filter.Operator == "IN" || filter.Operator == "NOT IN" || filter.Operator == "ALL" || filter.Operator == "<> ALL" || filter.Operator == "ANY" || filter.Operator == "<> ANY" {
			return fmt.Sprintf(" %s %s (%s)", left, filter.Operator, right), args, nil
		} else if filter.Operator == "?&" || filter.Operator == "?|" {
			switch filter.Right.(type) {
			case DialectStringerWithArgs, SqlUnsafe:
				break
			default:
				return fmt.Sprintf(" %s %s array[%s]", left, filter.Operator, right), args, nil
			}
		}
		return fmt.Sprintf(" %s %s %s", left, filter.Operator, right), args, nil
	}

	return "", args, fmt.Errorf("rem: invalid rule '%s' on WHERE clause", filter.Rule)
}

func And(clauses ...interface{}) []FilterClause {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	indent := 0
	filter := []FilterClause{{Rule: "("}}
	for i, clause := range flat {
		if i > 0 && indent == 0 {
			filter = append(filter, FilterClause{Rule: "AND"})
		}
		if clause.Rule == "(" {
			indent++
		} else if clause.Rule == ")" {
			indent--
		}
		filter = append(filter, clause)
	}
	return append(filter, FilterClause{Rule: ")"})
}

func Exists(value interface{}) FilterClause {
	return FilterClause{
		Left:     "",
		Operator: "EXISTS",
		Right:    value,
		Rule:     "WHERE",
	}
}

func flattenFilterClause(clauses []FilterClause, clause interface{}) []FilterClause {
	switch ct := clause.(type) {
	case FilterClause:
		clauses = append(clauses, ct)
	case []FilterClause:
		clauses = append(clauses, ct...)
	}
	return clauses
}

func NotExists(value interface{}) FilterClause {
	return FilterClause{
		Left:     "",
		Operator: "NOT EXISTS",
		Right:    value,
		Rule:     "WHERE",
	}
}

func Or(clauses ...interface{}) []FilterClause {
	flat := make([]FilterClause, 0)
	for _, clause := range clauses {
		flat = flattenFilterClause(flat, clause)
	}

	indent := 0
	filter := []FilterClause{{Rule: "("}}
	for i, clause := range flat {
		if i > 0 && indent == 0 {
			filter = append(filter, FilterClause{Rule: "OR"})
		}
		if clause.Rule == "(" {
			indent++
		} else if clause.Rule == ")" {
			indent--
		}
		filter = append(filter, clause)
	}
	return append(filter, FilterClause{Rule: ")"})
}

func Q(column interface{}, operator string, value interface{}) FilterClause {
	return FilterClause{
		Left:     column,
		Operator: operator,
		Right:    value,
		Rule:     "WHERE",
	}
}
