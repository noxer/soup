/* soup package implements a simple web scraper for Go,
keeping it as similar as possible to BeautifulSoup
*/

package soup

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// Root is a structure containing a pointer to an html node, the node value, and an error variable to return an error if occurred
type Root struct {
	Pointer   *html.Node
	NodeValue string
	Error     error
}

func wrapNode(n *html.Node) Root {

	return Root{
		Pointer:   n,
		NodeValue: n.Data,
	}

}

func wrapErr(err error) Root {
	return Root{Error: err}
}

func wrapErrf(format string, a ...interface{}) Root {
	return wrapErr(fmt.Errorf(format, a...))
}

// Headers contains all HTTP headers to send
var Headers = make(map[string]string)

// Cookies contains all HTTP cookies to send
var Cookies = make(map[string]string)

// Header sets a new HTTP header
func Header(n string, v string) {
	Headers[n] = v
}

// Cookie sets a cookie to send
func Cookie(n string, v string) {
	Cookies[n] = v
}

// GetWithClient returns the HTML returned by the url using a provided HTTP client
func GetWithClient(url string, client *http.Client) (string, error) {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", errors.New("couldn't perform GET request to " + url)
	}

	// Set headers
	for name, val := range Headers {
		req.Header.Set(name, val)
	}

	// Set cookies
	for name, val := range Cookies {
		req.AddCookie(&http.Cookie{
			Name:  name,
			Value: val,
		})
	}

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("couldn't perform GET request to " + url)
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("unable to read the response body")
	}

	return string(bytes), nil
}

// Get returns the HTML returned by the url in string using the default HTTP client
func Get(url string) (string, error) {
	return GetWithClient(url, http.DefaultClient)
}

// HTMLParse parses the HTML returning a start pointer to the DOM
func HTMLParse(s string) Root {

	r, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return wrapErrf("unable to parse the HTML")
	}

	for r.Type != html.ElementNode {
		switch r.Type {
		case html.DocumentNode:
			r = r.FirstChild
		case html.DoctypeNode:
			r = r.NextSibling
		case html.CommentNode:
			r = r.NextSibling
		}
	}

	return wrapNode(r)

}

// Find finds the first occurrence of the given tag name,
// with or without attribute key and value specified,
// and returns a struct with a pointer to it
func (r Root) Find(args ...string) Root {

	if len(args) == 0 {
		return wrapErrf("not enough arguments")
	}

	temp, ok := findOnce(r.Pointer, args, false, false)
	if !ok {
		return wrapErrf("element `%s` with attributes `%s` not found", args[0], strings.Join(args[1:], " "))
	}

	return wrapNode(temp)

}

// FindAll finds all occurrences of the given tag name,
// with or without key and value specified,
// and returns an array of structs, each having
// the respective pointers
func (r Root) FindAll(args ...string) []Root {

	temp := findAllofem(r.Pointer, args, false)
	if len(temp) == 0 {
		return nil
	}

	ps := make([]Root, len(temp))
	for i, t := range temp {
		ps[i] = wrapNode(t)
	}

	return ps
}

// FindStrict finds the first occurrence of the given tag name
// only if all the values of the provided attribute are an exact match
func (r Root) FindStrict(args ...string) Root {

	if len(args) == 0 {
		return wrapErrf("not enough arguments")
	}

	temp, ok := findOnce(r.Pointer, args, false, true)
	if !ok {
		return wrapErrf("element `%s` with attributes `%s` not found", args[0], strings.Join(args[1:], " "))
	}

	return wrapNode(temp)

}

// FindAllStrict finds all occurrences of the given tag name
// only if all the values of the provided attribute are an exact match
func (r Root) FindAllStrict(args ...string) []Root {

	temp := findAllofem(r.Pointer, args, true)
	if len(temp) == 0 {
		return nil
	}

	ps := make([]Root, len(temp))
	for i, t := range temp {
		ps[i] = wrapNode(t)
	}

	return ps

}

// FindNextSibling finds the next sibling of the pointer in the DOM
// returning a struct with a pointer to it
func (r Root) FindNextSibling() Root {

	next := r.Pointer.NextSibling
	if next == nil {
		return wrapErrf("no next sibling found")
	}
	return wrapNode(next)

}

// FindPrevSibling finds the previous sibling of the pointer in the DOM
// returning a struct with a pointer to it
func (r Root) FindPrevSibling() Root {

	prev := r.Pointer.PrevSibling
	if prev == nil {
		return wrapErrf("no previous sibling found")
	}
	return wrapNode(prev)
}

// FindNextElementSibling finds the next element sibling of the pointer in the DOM
// returning a struct with a pointer to it
func (r Root) FindNextElementSibling() Root {

	for k := r.Pointer.NextSibling; k != nil; k = k.NextSibling {

		if k.Type == html.ElementNode {
			return wrapNode(k)
		}

	}

	return wrapErrf("no next element sibling found")

}

// FindPrevElementSibling finds the previous element sibling of the pointer in the DOM
// returning a struct with a pointer to it
func (r Root) FindPrevElementSibling() Root {

	for k := r.Pointer.PrevSibling; k != nil; k = k.PrevSibling {

		if k.Type == html.ElementNode {
			return wrapNode(k)
		}

	}

	return wrapErrf("no previous element sibling found")

}

// Children retuns all direct children of this DOME element.
func (r Root) Children() []Root {

	var cs []Root

	for k := r.Pointer.FirstChild; k != nil; k = k.NextSibling {
		cs = append(cs, Root{k, k.Data, nil})
	}

	return cs

}

// Attrs returns a map containing all attributes
func (r Root) Attrs() map[string]string {

	if r.Pointer.Type != html.ElementNode {
		return nil
	}

	return getKeyValue(r.Pointer.Attr)

}

// Text returns the string inside a non-nested element
func (r Root) Text() string {

	// Fast path, the element itself is a `html.TextNode`
	if r.Pointer.Type == html.TextNode {
		return r.Pointer.Data
	}

	s := &strings.Builder{}

	for k := r.Pointer.FirstChild; k != nil; k = k.NextSibling {

		if k.Type == html.TextNode {
			s.WriteString(k.Data)
		}

	}

	return s.String()

}

// FullText combines all the text elements withing this element,
// including the nested elements.
func (r Root) FullText() string {

	s := &strings.Builder{}
	fullText(s, r.Pointer)
	return s.String()

}

func fullText(s *strings.Builder, n *html.Node) {

	if n.Type == html.TextNode {

		s.WriteString(n.Data)
		return

	}

	for k := n.FirstChild; k != nil; k = k.NextSibling {
		fullText(s, k)
	}

}

// Using depth first search to find the first occurrence and return
func findOnce(n *html.Node, args []string, uni bool, strict bool) (*html.Node, bool) {
	if uni == true {
		if n.Type == html.ElementNode && n.Data == args[0] {
			if len(args) > 1 && len(args) < 4 {
				for i := 0; i < len(n.Attr); i++ {
					attr := n.Attr[i]
					searchAttrName := args[1]
					searchAttrVal := args[2]
					if (strict && attributeAndValueEquals(attr, searchAttrName, searchAttrVal)) ||
						(!strict && attributeContainsValue(attr, searchAttrName, searchAttrVal)) {
						return n, true
					}
				}
			} else if len(args) == 1 {
				return n, true
			}
		}
	}
	uni = true
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p, q := findOnce(c, args, true, strict)
		if q != false {
			return p, q
		}
	}
	return nil, false
}

// Using depth first search to find all occurrences and return
func findAllofem(n *html.Node, args []string, strict bool) []*html.Node {

	var nodeLinks = make([]*html.Node, 0, 10)
	var f func(*html.Node, []string, bool)
	f = func(n *html.Node, args []string, uni bool) {
		if uni == true {
			if n.Data == args[0] {
				if len(args) > 1 && len(args) < 4 {
					for i := 0; i < len(n.Attr); i++ {
						attr := n.Attr[i]
						searchAttrName := args[1]
						searchAttrVal := args[2]
						if (strict && attributeAndValueEquals(attr, searchAttrName, searchAttrVal)) ||
							(!strict && attributeContainsValue(attr, searchAttrName, searchAttrVal)) {
							nodeLinks = append(nodeLinks, n)
						}
					}
				} else if len(args) == 1 {
					nodeLinks = append(nodeLinks, n)
				}
			}
		}
		uni = true
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c, args, true)
		}
	}
	f(n, args, false)
	return nodeLinks
}

// attributeAndValueEquals reports when the html.Attribute attr has the same attribute name and value as from
// provided arguments
func attributeAndValueEquals(attr html.Attribute, key, val string) bool {
	return attr.Key == key && attr.Val == val
}

// attributeContainsValue reports when the html.Attribute attr has the same attribute name as from provided
// attribute argument and compares if it has the same value in its values parameter
func attributeContainsValue(attr html.Attribute, key, val string) bool {

	if attr.Key != key {
		return false
	}

	fs := strings.Fields(attr.Val)
	for _, f := range fs {
		if f == val {
			return true
		}
	}

	return false

}

// Returns a key pair value (like a dictionary) for each attribute
func getKeyValue(attrs []html.Attribute) map[string]string {

	kv := make(map[string]string, len(attrs))

	for _, attr := range attrs {
		kv[attr.Key] = attr.Val
	}

	return kv

}
