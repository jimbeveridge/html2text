package html2text

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	spacingRe = regexp.MustCompile(`[ \r\n\t]+`)
	newlineRe = regexp.MustCompile(`\n\n+`)
)

type textifyTraverseCtx struct {
	Buf bytes.Buffer

	endsWithSpace bool
	PreviousNode  *html.Node
}

func (ctx *textifyTraverseCtx) setPreviousNode(previous *html.Node) {
	ctx.PreviousNode = previous
}

func (ctx *textifyTraverseCtx) Traverse(node *html.Node) error {

	defer ctx.setPreviousNode(node)

	switch node.Type {

	default:
		return ctx.TraverseChildren(node)

	case html.TextNode:
		//data := strings.Trim(spacingRe.ReplaceAllString(node.Data, " "), " ")
		data := spacingRe.ReplaceAllString(node.Data, " ")
		return ctx.Emit(data)

	case html.ElementNode:

		switch node.DataAtom {
		case atom.Br:
			return ctx.Emit("\n")

		case atom.Td:
			if ctx.PreviousNode.DataAtom != atom.Tr {
				ctx.Emit(" ")
			}
			return ctx.TraverseChildren(node)

		case atom.H1, atom.H2, atom.H3:
			subCtx := textifyTraverseCtx{}
			if err := subCtx.TraverseChildren(node); err != nil {
				return err
			}

			str := subCtx.Buf.String()
			dividerLen := 0
			for _, line := range strings.Split(str, "\n") {
				if lineLen := len([]rune(line)); lineLen-1 > dividerLen {
					dividerLen = lineLen - 1
				}
			}
			divider := ""
			if node.DataAtom == atom.H1 {
				divider = strings.Repeat("*", dividerLen)
			} else {
				divider = strings.Repeat("-", dividerLen)
			}

			if node.DataAtom == atom.H3 {
				return ctx.Emit("\n\n" + str + "\n" + divider + "\n\n")
			}
			return ctx.Emit("\n\n" + divider + "\n" + str + "\n" + divider + "\n\n")

		case atom.Li:
			if err := ctx.Emit("* "); err != nil {
				return err
			}

			if err := ctx.TraverseChildren(node); err != nil {
				return err
			}

			return ctx.Emit("\n")

		case atom.A:
			child := node.FirstChild
			hasSingleChild := child != nil && child == node.LastChild

			// If image is the only child, take its alt text as the link text
			if hasSingleChild && child.DataAtom == atom.Img {
				if altText := getAttrVal(child, "alt"); altText != "" {
					ctx.Emit(altText)
				}
			} else if err := ctx.TraverseChildren(node); err != nil {
				return err
			}

			// If there's a single inner text node, then get its text. We do not
			// want to output the link if it matches the inner text. This is
			// imperfect - we really want to compare the link to the text that
			// was output by TraverseChildren(), but it's not easily available.
			innerText := ""
			if hasSingleChild && child.Type == html.TextNode {
				innerText = node.FirstChild.Data
			}

			hrefLink := ""
			if attrVal := getAttrVal(node, "href"); attrVal != "" {
				attrVal = ctx.NormalizeHrefLink(attrVal)
				if attrVal != "" && attrVal != innerText {
					hrefLink = " ( " + attrVal + " )"
				}
			}

			return ctx.Emit(hrefLink)

		case atom.P, atom.Ul, atom.Table:
			if err := ctx.Emit("\n\n"); err != nil {
				return err
			}

			if err := ctx.TraverseChildren(node); err != nil {
				return err
			}

			return ctx.Emit("\n\n")

		case atom.Tr:
			if err := ctx.TraverseChildren(node); err != nil {
				return err
			}

			return ctx.Emit("\n")

		case atom.Style, atom.Script, atom.Head:
			// Ignore the subtree
			return nil

		default:
			return ctx.TraverseChildren(node)
		}
	}
}

func (ctx *textifyTraverseCtx) TraverseChildren(node *html.Node) error {
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if err := ctx.Traverse(c); err != nil {
			return err
		}
	}

	return nil
}

func (ctx *textifyTraverseCtx) Emit(data string) error {
	if len(data) == 0 {
		return nil
	}

	runes := []rune(data)
	//startsWithSpace := unicode.IsSpace(runes[0])
	//	if !startsWithSpace && !ctx.endsWithSpace {
	//		ctx.Buf.WriteByte(' ')
	//	}
	ctx.endsWithSpace = unicode.IsSpace(runes[len(runes)-1])

	_, err := ctx.Buf.WriteString(data)
	return err
}

func (ctx *textifyTraverseCtx) NormalizeHrefLink(link string) string {
	link = strings.TrimSpace(link)
	link = strings.TrimPrefix(link, "mailto:")
	return link
}

func getAttrVal(node *html.Node, attrName string) string {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val
		}
	}

	return ""
}

func FromReader(reader io.Reader) (string, error) {
	doc, err := html.Parse(reader)
	if err != nil {
		return "", err
	}

	ctx := textifyTraverseCtx{
		Buf: bytes.Buffer{},
	}
	if err = ctx.Traverse(doc); err != nil {
		return "", err
	}

	text := strings.TrimSpace(newlineRe.ReplaceAllString(
		strings.Replace(ctx.Buf.String(), "\n ", "\n", -1), "\n\n"))
	return text, nil
}

func FromString(input string) (string, error) {
	text, err := FromReader(strings.NewReader(input))
	if err != nil {
		return "", err
	}
	return text, nil
}
