package main

import (
	"bytes"
	"cmp"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

//go:embed index.html.template
var indexHTMLTemplate string

func main() {
	table := Table{ColumnCount: 10, RowCount: 10}
	flag.IntVar(&table.ColumnCount, "columns", table.ColumnCount, "the number of table columns")
	flag.IntVar(&table.RowCount, "rows", table.RowCount, "the number of table rows")
	flag.Parse()
	s := server{
		table:     table,
		templates: template.Must(template.New("index.html.template").Parse(indexHTMLTemplate)),
	}
	log.Println("starting server")
	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), s.routes()))
}

type server struct {
	table Table
	mut   sync.RWMutex

	templates *template.Template
}

func (server *server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("GET /table.json", server.getTableJSON)
	mux.HandleFunc("POST /table.json", server.postTableJSON)
	mux.HandleFunc("GET /cell/{id}", server.getCellEdit)
	mux.HandleFunc("PATCH /table", server.patchTable)

	return mux
}

func (server *server) render(res http.ResponseWriter, _ *http.Request, name string, status int, data any) {
	var buf bytes.Buffer
	if err := server.templates.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	header := res.Header()
	header.Set("content-type", "text/html")
	res.WriteHeader(status)
	_, _ = res.Write(buf.Bytes())
}

func (server *server) index(res http.ResponseWriter, req *http.Request) {
	server.mut.RLock()
	defer server.mut.RUnlock()
	server.render(res, req, "index.html.template", http.StatusOK, &server.table)
}

func (server *server) getCellEdit(res http.ResponseWriter, req *http.Request) {
	server.mut.RLock()
	defer server.mut.RUnlock()

	column, row, err := parseCellID(req.PathValue("id"), server.table.ColumnCount-1, server.table.RowCount-1)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	cell := server.table.Cell(column, row)
	server.render(res, req, "edit-cell", http.StatusOK, cell)
}

func (server *server) getTableJSON(res http.ResponseWriter, _ *http.Request) {
	server.mut.RLock()
	defer server.mut.RUnlock()

	filtered := server.table.Cells[:0]
	for _, cell := range server.table.Cells {
		if cell.SavedExpression == nil || cell.Expression == nil {
			continue
		}
		filtered = append(filtered, cell)
	}
	server.table.Cells = filtered

	buf, err := json.MarshalIndent(server.table, "", "\t")
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	h := res.Header()
	h.Set("content-type", "application/json")
	h.Set("content-length", strconv.Itoa(len(buf)))
	res.WriteHeader(http.StatusOK)
	_, _ = res.Write(buf)
}

func (server *server) postTableJSON(res http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm((1 << 10) * 10); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	tableJSONHeaders, ok := req.MultipartForm.File["table.json"]
	if !ok || len(tableJSONHeaders) == 0 {
		http.Error(res, "expected table.json file", http.StatusBadRequest)
		return
	}
	f, err := tableJSONHeaders[0].Open()
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	defer closeAndIgnoreError(f)
	tableJSON, err := io.ReadAll(f)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	var table Table
	if err = json.Unmarshal(tableJSON, &table); err != nil {
		log.Fatal(err)
	}
	if err := table.calculateValues(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.mut.Lock()
	defer server.mut.Unlock()
	server.table = table

	server.render(res, req, "table", http.StatusOK, &server.table)
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}

func (server *server) patchTable(res http.ResponseWriter, req *http.Request) {
	server.mut.Lock()
	defer server.mut.Unlock()

	if err := req.ParseForm(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	for key, value := range req.Form {
		if !strings.HasPrefix(key, "cell-") {
			continue
		}
		column, row, err := parseCellID(key, server.table.ColumnCount-1, server.table.RowCount-1)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		cell := server.cellPointer(column, row)
		cell.Error = ""
		cell.input = normalizeExpression(value[0])

		var expression ExpressionNode
		if cell.input != "" {
			expression, err = newExpression(cell.input, server.table.ColumnCount-1, server.table.RowCount-1)
			if err != nil {
				cell.Error = err.Error()
				continue
			}
			cell.input = expression.String()
		}
		cell.Expression = expression
	}

	err := server.table.calculateValues()
	if err != nil {
		server.render(res, req, "table", http.StatusOK, &server.table)
		return
	}

	server.render(res, req, "table", http.StatusOK, &server.table)
}

func (server *server) cellPointer(column, row int) *Cell {
	var cell *Cell
	index := slices.IndexFunc(server.table.Cells, func(cell Cell) bool {
		return cell.Row == row && cell.Column == column
	})
	if index >= 0 {
		cell = &server.table.Cells[index]
	} else {
		server.table.Cells = append(server.table.Cells, Cell{
			Row:    row,
			Column: column,
		})
		cell = &server.table.Cells[len(server.table.Cells)-1]
	}
	return cell
}

func normalizeExpression(in string) string {
	return strings.TrimSpace(strings.ToUpper(in))
}

type Column struct {
	Number int
}

func (column Column) Label() string {
	return columnLabel(column.Number)
}

func columnLabel(n int) string {
	result := ""
	for n >= 0 {
		remainder := n % 26
		result = fmt.Sprintf("%c", remainder+65) + result
		n = n/26 - 1
	}
	return result
}

func columnNumber(label string) int {
	result := 0
	for _, char := range label {
		result = result*26 + int(char) - 64
	}
	return result - 1
}

type Row struct {
	Number int
}

func (row Row) Label() string {
	return strconv.Itoa(row.Number)
}

type Cell struct {
	Row    int
	Column int

	Expression,
	SavedExpression ExpressionNode
	Value,
	SavedValue int

	input,
	Error string
}

func (cell *Cell) ExpressionText() string {
	if cell.Expression != nil && cell.Error == "" {
		return cell.Expression.String()
	}
	return cell.input
}

type EncodedCell struct {
	ID         string `json:"id"`
	Expression string `json:"ex"`
}

func (cell *Cell) MarshalJSON() ([]byte, error) {
	return json.Marshal(EncodedCell{
		ID:         strings.TrimPrefix(cell.ID(), "cell-"),
		Expression: cell.SavedExpression.String(),
	})
}

type EncodedTable struct {
	ColumnCount int           `json:"columns"`
	RowCount    int           `json:"rows"`
	Cells       []EncodedCell `json:"cells"`
}

func (table *Table) UnmarshalJSON(in []byte) error {
	var encoded EncodedTable

	if err := json.Unmarshal(in, &encoded); err != nil {
		return err
	}
	table.RowCount = encoded.RowCount
	table.ColumnCount = encoded.ColumnCount
	for _, cell := range encoded.Cells {
		column, row, err := parseCellID(cell.ID, table.ColumnCount-1, table.RowCount-1)
		if err != nil {
			return err
		}
		exp, err := newExpression(cell.Expression, table.ColumnCount-1, table.RowCount-1)
		if err != nil {
			return err
		}
		table.Cells = append(table.Cells, Cell{
			Column:          column,
			Row:             row,
			SavedExpression: exp,
			Expression:      exp,
		})
	}

	return table.calculateValues()
}

func (cell *Cell) String() string {
	if cell.SavedExpression == nil {
		return ""
	}
	return strconv.Itoa(cell.Value)
}

func (cell *Cell) IDPathParam() string {
	return fmt.Sprintf("%s%d", columnLabel(cell.Column), cell.Row)
}

func (cell *Cell) ID() string {
	return "cell-" + cell.IDPathParam()
}

type Table struct {
	ColumnCount int    `json:"columns"`
	RowCount    int    `json:"rows"`
	Cells       []Cell `json:"cells"`
}

func NewTable(columns, rows int) Table {
	table := Table{
		RowCount:    rows,
		ColumnCount: columns,
	}
	return table
}

func (table *Table) Cell(column, row int) *Cell {
	for i, cell := range table.Cells {
		if cell.Row == row && cell.Column == column {
			return &table.Cells[i]
		}
	}
	return &Cell{
		Row:    row,
		Column: column,
	}
}

func (table *Table) Rows() []Row {
	result := make([]Row, table.RowCount)
	for i := range result {
		result[i].Number = i
	}
	return result
}

func (table *Table) Columns() []Column {
	result := make([]Column, table.ColumnCount)
	for i := range result {
		result[i].Number = i
	}
	return result
}

func (table *Table) calculateValues() error {
	for _, cell := range table.Cells {
		if cell.Error != "" {
			return fmt.Errorf("cell parsing error %s", cell.IDPathParam())
		}
	}
	for i := range table.Cells {
		visited := make(visitSet)
		cell := &table.Cells[i]
		err := cell.evaluate(table, visited)
		if err != nil {
			cell.Error = err.Error()
			table.revertCellChanges()
			return err
		}
		cell.Error = ""
	}
	table.saveCellChanges()
	return nil
}

var identifierPattern = regexp.MustCompile("(?P<column>[A-Z]+)(?P<row>[0-9]+)")

func parseCellID(in string, maxColumn, maxRow int) (int, int, error) {
	in = strings.TrimPrefix(in, "cell-")
	if !identifierPattern.MatchString(in) {
		return 0, 0, fmt.Errorf("unexpected identifier pattern expected something like A4")
	}
	parts := identifierPattern.FindStringSubmatch(in)
	columnName := parts[identifierPattern.SubexpIndex("column")]
	row, err := strconv.Atoi(parts[identifierPattern.SubexpIndex("row")])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse row number: %w", err)
	}
	if row > maxRow {
		return 0, 0, fmt.Errorf("row number %d out of range it must be greater than 0 and less than or equal to %d", row, maxRow)
	}
	column := columnNumber(columnName)
	if column > maxColumn {
		return 0, 0, fmt.Errorf("column %s out of range it must be greater than or equal to %s and less than or equal to %s", columnName, columnLabel(0), columnLabel(maxColumn))
	}
	return column, row, nil
}

func (table *Table) saveCellChanges() {
	for i := range table.Cells {
		table.Cells[i].SavedValue = table.Cells[i].Value
		table.Cells[i].SavedExpression = table.Cells[i].Expression
	}
}

func (table *Table) revertCellChanges() {
	for i := range table.Cells {
		table.Cells[i].Value = table.Cells[i].SavedValue
		table.Cells[i].Expression = table.Cells[i].SavedExpression
	}
}

type Token struct {
	Type  TokenType
	Value string
	Index int
}

func (token Token) BinaryOpLess(other Token) bool {
	return token.Type < other.Type
}

type TokenType int

const (
	TokenNumber TokenType = iota
	TokenAdd
	TokenSubtract
	TokenMultiply
	TokenDivide
	TokenExponent
	TokenExclamation
	TokenLeftParenthesis
	TokenRightParenthesis
	TokenIdentifier
)

func tokenize(input string) ([]Token, error) {
	var tokens []Token

	for i := 0; i < len(input); i++ {
		c := rune(input[i])

		if unicode.IsDigit(c) {
			start := i
			dotCount := 0
			for i < len(input) && (unicode.IsDigit(rune(input[i])) || (dotCount == 0 && input[i] == '.')) {
				if input[i] == '.' {
					dotCount++
				}
				i++
			}
			tokens = append(tokens, Token{Index: start, Type: TokenNumber, Value: input[start:i]})
			i--
		} else if c == '+' {
			tokens = append(tokens, Token{Index: i, Type: TokenAdd, Value: "+"})
		} else if c == '!' {
			tokens = append(tokens, Token{Index: i, Type: TokenExclamation, Value: "!"})
		} else if c == '-' {
			tokens = append(tokens, Token{Index: i, Type: TokenSubtract, Value: "-"})
		} else if c == '*' {
			tokens = append(tokens, Token{Index: i, Type: TokenMultiply, Value: "*"})
		} else if c == '/' {
			tokens = append(tokens, Token{Index: i, Type: TokenDivide, Value: "/"})
		} else if c == '^' {
			tokens = append(tokens, Token{Index: i, Type: TokenExponent, Value: "^"})
		} else if c == '(' {
			tokens = append(tokens, Token{Index: i, Type: TokenLeftParenthesis, Value: "("})
		} else if c == ')' {
			tokens = append(tokens, Token{Index: i, Type: TokenRightParenthesis, Value: ")"})
		} else if unicode.IsSpace(c) {
			continue
		} else if unicode.IsLetter(rune(input[i])) {
			start := i
			for i < len(input) && (rune(input[i]) == '_' || unicode.IsLetter(rune(input[i])) || unicode.IsDigit(rune(input[i]))) {
				i++
			}
			tokens = append(tokens, Token{Index: start, Type: TokenIdentifier, Value: input[start:i]})
			i--
		}
	}

	return tokens, nil
}

type ExpressionNode interface {
	fmt.Stringer
}

func newExpression(in string, maxColumn, maxRow int) (ExpressionNode, error) {
	expressionText := normalizeExpression(in)
	tokens, err := tokenize(expressionText)
	if err != nil {
		return nil, err
	}
	expression, _, err := parse(tokens, 0, maxColumn, maxRow)
	if err != nil {
		return nil, err
	}
	return expression, nil
}

type IdentifierNode struct {
	Token Token

	Row, Column int
}

func (node IdentifierNode) String() string {
	return node.Token.Value
}

type IntegerNode struct {
	Token Token
	Value int
}

func (node IntegerNode) String() string {
	return node.Token.Value
}

type BinaryExpressionNode struct {
	Op          Token
	Left, Right ExpressionNode
}

func (node BinaryExpressionNode) String() string {
	return fmt.Sprintf("%s %s %s", node.Left.String(), node.Op.Value, node.Right.String())
}

type VariableNode struct {
	Identifier Token
}

func (node VariableNode) String() string {
	return fmt.Sprintf("%s", node.Identifier.Value)
}

type FactorialNode struct {
	Expression ExpressionNode
}

func (node FactorialNode) String() string {
	return fmt.Sprintf("%s!", node.Expression)
}

type ParenNode struct {
	Start, End Token
	Node       ExpressionNode
}

func (node ParenNode) String() string {
	return fmt.Sprintf("(%s)", node.Node)
}

func parse(tokens []Token, i, maxColumn, maxRow int) (ExpressionNode, int, error) {
	var stack []ExpressionNode
	for {
		result, consumed, err := parseNodes(stack, tokens, i, maxColumn, maxRow)
		if err != nil {
			return nil, consumed + i, err
		}
		i += consumed
		stack = result
		if i < len(tokens) {
			continue
		}
		if len(stack) < 1 {
			return nil, i, fmt.Errorf("parsing failed to return an expression")
		}
		if len(stack) > 1 {
			return nil, i, fmt.Errorf("failed build parse tree multiple %d nodes still on stack: %#v", len(stack)-1, stack)
		}
		return stack[0], i, nil
	}
}

func parseNodes(stack []ExpressionNode, tokens []Token, i, maxColumn, maxRow int) ([]ExpressionNode, int, error) {
	if i >= len(tokens) {
		return nil, i, nil
	}

	token := tokens[i]

	switch token.Type {
	case TokenNumber:
		n, err := strconv.Atoi(token.Value)
		if err != nil {
			return nil, 1, fmt.Errorf("failed to parse number  %s at expression offset %d: %w", token.Value, token.Index, err)
		}
		return append(stack, IntegerNode{Token: token, Value: n}), 1, nil
	case TokenIdentifier:
		switch token.Value {
		case RowIdent, ColumnIdent, MaxRowIdent, MaxColumnIdent, MinRowIdent, MinColumnIdent:
			return append(stack, VariableNode{Identifier: token}), 1, nil
		default:
			column, row, err := parseCellID(token.Value, maxColumn, maxRow)
			if err != nil {
				return nil, 0, err
			}
			return append(stack, IdentifierNode{Token: token, Row: row, Column: column}), 1, nil
		}
	case TokenLeftParenthesis:
		var (
			totalConsumed = 1
			parenStack    []ExpressionNode
		)
		i += 1
		for {
			result, consumed, err := parseNodes(parenStack, tokens, i, maxColumn, maxRow)
			if err != nil {
				return nil, 0, err
			}
			totalConsumed += consumed
			i += consumed
			if i >= len(tokens) {
				return nil, 0, fmt.Errorf("parenthesis at expression offset %d is missing closing parenthesis", token.Index)
			}
			if tokens[i].Type != TokenRightParenthesis {
				parenStack = result
				continue
			}
			if len(result) == 0 {
				return nil, 0, fmt.Errorf("parentheses expression is empty")
			}
			return append(stack, ParenNode{
				Node: result[0],
			}), totalConsumed + 1, nil
		}
	case TokenExclamation:
		if len(stack) == 0 {
			return nil, 0, fmt.Errorf("malformed factorial expression")
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if b, ok := top.(BinaryExpressionNode); ok {
			if b.Op.BinaryOpLess(token) {
				return append(stack, BinaryExpressionNode{
					Op:   b.Op,
					Left: b.Left,
					Right: FactorialNode{
						Expression: b.Right,
					},
				}), 1, nil
			}
		}

		stack = append(stack, FactorialNode{
			Expression: top,
		})
		return stack, 1, nil
	case TokenAdd, TokenSubtract, TokenMultiply, TokenDivide, TokenExponent:
		node := BinaryExpressionNode{
			Op: token,
		}

		if len(stack) == 0 {
			if token.Type != TokenSubtract {
				return stack, 0, fmt.Errorf("binary expression for operator at index %d missing left hand side", token.Index)
			}
			node.Left = IntegerNode{Value: 0}
		} else {
			node.Left = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}

		rightExpression, consumed, err := parseNodes(nil, tokens, i+1, maxColumn, maxRow)
		if err != nil {
			return nil, 1 + consumed, err
		}
		if len(rightExpression) != 1 {
			return stack, 0, fmt.Errorf("weird right hand expression after operator at offet %d", token.Index)
		}
		node.Right = rightExpression[0]

		if leftBinNode, ok := node.Left.(BinaryExpressionNode); ok {
			if leftBinNode.Op.BinaryOpLess(node.Op) {
				leftLeft := leftBinNode.Left
				leftRight := leftBinNode.Right
				rightNode := node.Right

				return append(stack, BinaryExpressionNode{
					Op:   leftBinNode.Op,
					Left: leftLeft,
					Right: BinaryExpressionNode{
						Op:    token,
						Left:  leftRight,
						Right: rightNode,
					},
				}), 1 + consumed, nil
			}
		}

		return append(stack, node), 1 + consumed, nil
	case TokenRightParenthesis:
		return nil, 0, fmt.Errorf("unexpected right parenthesis at expression offest %d", token.Index)
	}

	return nil, 0, nil
}

type visit struct {
	colum, row int
}

type visitSet map[visit]struct{}

func (cell *Cell) evaluate(table *Table, visited visitSet) error {
	v := visit{
		colum: cell.Column,
		row:   cell.Row,
	}
	_, alreadyVisited := visited[v]
	if alreadyVisited {
		return fmt.Errorf("recursive reference to %s%d", columnLabel(cell.Column), cell.Row)
	}
	visited[v] = struct{}{}
	if cell.Expression == nil {
		cell.Value = 0
		return nil
	}
	result, err := evaluate(table, cell, visited, cell.Expression)
	if err != nil {
		return err
	}
	cell.Value = result
	return nil
}

const (
	RowIdent       = "ROW"
	ColumnIdent    = "COLUMN"
	MaxRowIdent    = "MAX_ROW"
	MaxColumnIdent = "MAX_COLUMN"
	MinRowIdent    = "MIN_ROW"
	MinColumnIdent = "MIN_COLUMN"
)

func evaluate(table *Table, cell *Cell, visited visitSet, expressionNode ExpressionNode) (int, error) {
	switch node := expressionNode.(type) {
	case IdentifierNode:
		cell := table.Cell(node.Column, node.Row)
		err := cell.evaluate(table, visited)
		return cell.Value, err
	case IntegerNode:
		return node.Value, nil
	case ParenNode:
		return evaluate(table, cell, visited, node.Node)
	case VariableNode:
		switch node.Identifier.Value {
		case RowIdent:
			return cell.Row, nil
		case ColumnIdent:
			return cell.Column, nil
		case MaxRowIdent:
			return table.RowCount - 1, nil
		case MaxColumnIdent:
			return table.ColumnCount - 1, nil
		case MinRowIdent, MinColumnIdent:
			return 0, nil
		default:
			return 0, fmt.Errorf("unknown variable %s", node.Identifier.Value)
		}
	case FactorialNode:
		n, err := evaluate(table, cell, visited, node.Expression)
		if err != nil {
			return 0, err
		}
		if n > 20 {
			return 0, fmt.Errorf("n! where n > 20 is too large")
		}
		for i := n - 1; i >= 2; i-- {
			n *= i
		}
		return n, nil
	case BinaryExpressionNode:
		leftResult, err := evaluate(table, cell, visited, node.Left)
		if err != nil {
			return 0, err
		}
		rightResult, err := evaluate(table, cell, visited, node.Right)
		if err != nil {
			return 0, err
		}
		switch node.Op.Type {
		case TokenAdd:
			return leftResult + rightResult, nil
		case TokenSubtract:
			return leftResult - rightResult, nil
		case TokenMultiply:
			return leftResult * rightResult, nil
		case TokenExponent:
			res := 1
			for i := 0; i < rightResult; i++ {
				res *= leftResult
			}
			return res, nil
		case TokenDivide:
			if rightResult == 0 {
				return 0, fmt.Errorf("could not divide by zero")
			}
			return leftResult / rightResult, nil
		default:
			return 0, fmt.Errorf("unknown binary operator %s", node.Op.Value)
		}
	default:
		return 0, fmt.Errorf("unknown expression node")
	}
}
