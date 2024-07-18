# goXtree

GoXtree is currently just a proof of concept library to deliver a convenient way to build WASM based webapps in Go.

You will find usage examples here: https://github.com/jarekjaryszew/goxtree_examples . As I am not tagging this yet you need to clone example repo aside this one.
```
your_dir
|- goxtree
|- goxtree_examples
```

Please mind that this is just a concept:
- It was tested only with the existing example.
- It does not provide proper resource cleanup.
- It uses syscall/js to integrate with JS API which unfortunately is not easy to work with. Most mistakes result in a panic killing entire webassembly app so making it stable seems like a greatest challenge.

## Concept

### Virtual DOM
As many SPA frameworks goXtree keeps its own representation of DOM tree and aplies changes in batches when requested.


### Declarative description of HTML elements
Each subtree is represented in a declarative format with html attributes as structure tags:
```go
type MyRoot struct {
	me any `tag:"div" id:"myroot"`
	_  any `tag:"h1" text:"Hello World!" class:"header" id:"head1"`
	_  any `tag:"div" id:"secondDiv"`
	_  struct {
		_ any `tag:"button" text:"Hi" class:"btn" id:"btn1"`
		_ any `tag:"button" text:"Bye" class:"btn" id:"btn2"`
	} `tag:"div" id:"innerDiv"`
}
```
Process of converting the template structure to virtual DOM is called dressing (like dressing a christmas tree). Dressed subtree may then be mounted.
```go
rootNode, _ := goxtree.DressDomTree(&MyRoot{})
rootNode.MountToTag("root")
```
Above code is enough to display a html document.
### Subtrees
GoXtree operates on smaller subtries which internally have immutable structure (but you can manipulate attributes and content) that are accessed via CoreNode. Subtrees may attach to arbitrary existing DOM node as long as it has an id field like: `<body id="root"></body>` or another subtree.
```go
type MyButton struct {
	me any `tag:"button" text:"backend" class:"btn" id:"btn1"`
}
// ...
rootNode.AddChildToElementWithId("secondDiv", buttonNode)
```
Event listener functions are added to DOM and registered with JS when subtree is mounted.
```go
	cb2 := func() {
        // Goroutine is used not to block js event loop.
		go func() {
			rootNode.SetTextToElementWithId("head1", "Hi")
			rootNode.Render()
		}()
	}
	rootNode.AddEventListenerToElementWithId("btn2", "click", cb2)
```
### Wrapping JS fetch API
It would be easiest to just use `net/http` but this import adds over 6MB to the WASM binary so the objective is to wrap the native fetch API.
```go
cbf := func(res string) {
    fmt.Print("fetch callback ", res)
    var r Response
    err := json.Unmarshal([]byte(res), &r)
    if err != nil {
        fmt.Println("error", err)
        return
    }
    rootNode.SetTextToElementWithId("head1", r.Message)
    rootNode.Render()
}
goxtree.Fetch("something.json", cbf, nil)
```
