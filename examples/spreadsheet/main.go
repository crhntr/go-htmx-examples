package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/julienschmidt/httprouter"
)

//go:embed index.html.template
var indexHTMLTemplate string

func main() {

	var (
		numericType           string
		rowCount, columnCount = 20, 10
	)
	flag.StringVar(&numericType, "type", "int", "the numeric type")
	flag.IntVar(&columnCount, "columns", columnCount, "the number of table columns")
	flag.IntVar(&rowCount, "rows", rowCount, "the number of table rows")
	flag.Parse()

	mux := constructRouter(numericType, columnCount, rowCount)

	log.Println("starting server")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

type Number interface {
	~float32 | ~float64 |
		~int | ~int64 | ~int32 | ~int16 | ~int8 |
		~uint | ~uint64 | ~uint32 | ~uint16 | ~uint8
}

func newServer[N Number](columnCount, rowCount int, parseNumber func(string) (N, error)) *httprouter.Router {
	table := Table[N]{ColumnCount: columnCount, RowCount: rowCount}
	table.ParseNumber = parseNumber
	s := server[N]{
		table:     table,
		templates: template.Must(template.New("index.html.template").Parse(indexHTMLTemplate)),
	}
	return s.routes()
}

func constructRouter(numericTypeName string, columnCount, rowCount int) *httprouter.Router {
	switch numericTypeName {
	case "float32":
		return newServer(columnCount, rowCount, func(in string) (float32, error) {
			n, err := strconv.ParseFloat(in, 32)
			return float32(n), err
		})
	case "float64":
		return newServer(columnCount, rowCount, func(in string) (float64, error) {
			return strconv.ParseFloat(in, 64)
		})
	case "int64":
		return newServer(columnCount, rowCount, parseInt[int64])
	case "int32":
		return newServer(columnCount, rowCount, parseInt[int32])
	case "int16":
		return newServer(columnCount, rowCount, parseInt[int16])
	case "int8":
		return newServer(columnCount, rowCount, parseInt[int8])
	case "uint":
		return newServer(columnCount, rowCount, parseInt[uint])
	case "uint64":
		return newServer(columnCount, rowCount, parseInt[uint64])
	case "uint32":
		return newServer(columnCount, rowCount, parseInt[uint32])
	case "uint16":
		return newServer(columnCount, rowCount, parseInt[uint16])
	case "uint8":
		return newServer(columnCount, rowCount, parseInt[uint8])
	case "int":
		return newServer(columnCount, rowCount, strconv.Atoi)
	default:
		panic("unknown table number type")
	}
}

func parseInt[T Number](s string) (T, error) {
	n, err := strconv.Atoi(s)
	return T(n), err
}

type server[N Number] struct {
	table Table[N]
	mut   sync.RWMutex

	templates *template.Template
}

func (server *server[N]) routes() *httprouter.Router {
	mux := httprouter.New()

	mux.GET("/", server.index)
	mux.GET("/table.json", server.getTableJSON)
	mux.POST("/table.json", server.postTableJSON)
	mux.GET("/cell/:id", server.getCellEdit)
	mux.PATCH("/table", server.patchTable)

	return mux
}

func (server *server[N]) render(res http.ResponseWriter, _ *http.Request, name string, status int, data any) {
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

func (server *server[N]) index(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	server.mut.RLock()
	defer server.mut.RUnlock()
	server.render(res, req, "index.html.template", http.StatusOK, &server.table)
}

func (server *server[N]) getCellEdit(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	server.mut.RLock()
	defer server.mut.RUnlock()

	column, row, err := parseCellID(params.ByName("id"), server.table.ColumnCount-1, server.table.RowCount-1)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	cell := server.table.Cell(column, row)
	server.render(res, req, "edit-cell", http.StatusOK, cell)
}

func (server *server[N]) getTableJSON(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
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

func (server *server[N]) postTableJSON(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
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
	var table Table[N]
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

func (server *server[N]) patchTable(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
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
			expression, err = newExpression(cell.input, server.table.ColumnCount-1, server.table.RowCount-1, server.table.ParseNumber)
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

func (server *server[N]) cellPointer(column, row int) *Cell[N] {
	var cell *Cell[N]
	index := slices.IndexFunc(server.table.Cells, func(cell Cell[N]) bool {
		return cell.Row == row && cell.Column == column
	})
	if index >= 0 {
		cell = &server.table.Cells[index]
	} else {
		server.table.Cells = append(server.table.Cells, Cell[N]{
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

type Cell[N Number] struct {
	Row    int
	Column int

	Expression,
	SavedExpression ExpressionNode
	Value,
	SavedValue N

	input,
	Error string
}

func (cell Cell[N]) ExpressionText() string {
	if cell.Expression != nil && cell.Error == "" {
		return cell.Expression.String()
	}
	return cell.input
}

type EncodedCell struct {
	ID         string `json:"id"`
	Expression string `json:"ex"`
}

func (cell Cell[N]) MarshalJSON() ([]byte, error) {
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

func (table *Table[N]) UnmarshalJSON(in []byte) error {
	var encoded EncodedTable

	if err := json.Unmarshal(in, &encoded); err != nil {
		return err
	}
	table.RowCount = encoded.RowCount
	table.ColumnCount = encoded.ColumnCount
	for _, cell := range encoded.Cells {
		column, row, err := parseCellID(cell.ID, table.RowCount-1, table.ColumnCount-1)
		if err != nil {
			return err
		}
		exp, err := newExpression(cell.Expression, table.RowCount-1, table.ColumnCount-1, table.ParseNumber)
		if err != nil {
			return err
		}
		table.Cells = append(table.Cells, Cell[N]{
			Column:          column,
			Row:             row,
			SavedExpression: exp,
			Expression:      exp,
		})
	}

	return table.calculateValues()
}

func (cell Cell[N]) String() string {
	if cell.SavedExpression == nil {
		return ""
	}
	return fmt.Sprintf("%v", cell.Value)
}

func (cell Cell[N]) IDPathParam() string {
	return fmt.Sprintf("%s%d", columnLabel(cell.Column), cell.Row)
}
func (cell Cell[N]) ID() string {
	return "cell-" + cell.IDPathParam()
}

type Table[N Number] struct {
	ColumnCount int       `json:"columns"`
	RowCount    int       `json:"rows"`
	Cells       []Cell[N] `json:"cells"`

	ParseNumber func(string) (N, error)
}

func NewTable[N Number](columns, rows int) Table[N] {
	table := Table[N]{
		RowCount:    rows,
		ColumnCount: columns,
	}
	return table
}

func (table *Table[N]) Cell(column, row int) Cell[N] {
	for _, cell := range table.Cells {
		if cell.Row == row && cell.Column == column {
			return cell
		}
	}
	return Cell[N]{
		Row:    row,
		Column: column,
	}
}

func (table *Table[N]) Rows() []Row {
	result := make([]Row, table.RowCount)
	for i := range result {
		result[i].Number = i
	}
	return result
}

func (table *Table[N]) Columns() []Column {
	result := make([]Column, table.ColumnCount)
	for i := range result {
		result[i].Number = i
	}
	return result
}

func (table *Table[N]) calculateValues() error {
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

func parseCellID(in string, maxRow, maxColumn int) (int, int, error) {
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

func (table *Table[N]) saveCellChanges() {
	for i := range table.Cells {
		table.Cells[i].SavedValue = table.Cells[i].Value
		table.Cells[i].SavedExpression = table.Cells[i].Expression
	}
}

func (table *Table[N]) revertCellChanges() {
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

func newExpression[N Number](in string, maxColumn, maxRow int, parseNumber func(string) (N, error)) (ExpressionNode, error) {
	expressionText := normalizeExpression(in)
	tokens, err := tokenize(expressionText)
	if err != nil {
		return nil, err
	}
	expression, _, err := parse(tokens, 0, maxRow, maxColumn, parseNumber)
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

type NumberNode[N Number] struct {
	Token Token
	Value N
}

func (node NumberNode[N]) String() string {
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

func parse[N Number](tokens []Token, i, maxRow, maxColumn int, parseNumber func(string) (N, error)) (ExpressionNode, int, error) {
	var (
		stack []ExpressionNode
	)
	for {
		result, consumed, err := parseNodes[N](stack, tokens, i, maxRow, maxColumn, parseNumber)
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

func parseNodes[N Number](stack []ExpressionNode, tokens []Token, i, maxRow, maxColumn int, parseNumber func(in string) (N, error)) ([]ExpressionNode, int, error) {
	if i >= len(tokens) {
		return nil, i, nil
	}

	token := tokens[i]

	switch token.Type {
	case TokenNumber:
		n, err := parseNumber(token.Value)
		if err != nil {
			return nil, 1, fmt.Errorf("failed to parse number  %s at expression offset %d: %w", token.Value, token.Index, err)
		}
		return append(stack, NumberNode[N]{Token: token, Value: n}), 1, nil
	case TokenIdentifier:
		switch token.Value {
		case "ROW", "COLUMN", "MAX_ROW", "MAX_COLUMN", "MIN_ROW", "MIN_COLUMN":
			return append(stack, VariableNode{Identifier: token}), 1, nil
		default:
			column, row, err := parseCellID(token.Value, maxRow, maxColumn)
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
			result, consumed, err := parseNodes(parenStack, tokens, i, maxRow, maxColumn, parseNumber)
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
	case TokenAdd, TokenSubtract, TokenMultiply, TokenDivide:
		node := BinaryExpressionNode{
			Op: token,
		}

		if len(stack) == 0 {
			if token.Type != TokenSubtract {
				return stack, 0, fmt.Errorf("binary expression for operator at index %d missing left hand side", token.Index)
			}
			node.Left = NumberNode[N]{Value: 0}
		} else {
			node.Left = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}

		rightExpression, consumed, err := parseNodes(nil, tokens, i+1, maxRow, maxColumn, parseNumber)
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

func (cell *Cell[N]) evaluate(table *Table[N], visited visitSet) error {
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

func evaluate[N Number](table *Table[N], cell *Cell[N], visited visitSet, expressionNode ExpressionNode) (N, error) {
	switch node := expressionNode.(type) {
	case IdentifierNode:
		cell := table.Cell(node.Column, node.Row)
		err := cell.evaluate(table, visited)
		return cell.Value, err
	case NumberNode[N]:
		return node.Value, nil
	case ParenNode:
		return evaluate(table, cell, visited, node.Node)
	case VariableNode:
		switch node.Identifier.Value {
		case "ROW":
			return N(cell.Row), nil
		case "COLUMN":
			return N(cell.Column), nil
		case "MAX_ROW":
			return N(table.RowCount) - 1, nil
		case "MAX_COLUMN":
			return N(table.ColumnCount) - 1, nil
		case "MIN_ROW", "MIN_COLUMN":
			return 0, nil
		default:
			return 0, fmt.Errorf("unknown variable %s", node.Identifier.Value)
		}
	case FactorialNode:
		n, err := evaluate(table, cell, visited, node.Expression)
		if err != nil {
			return 0, err
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
