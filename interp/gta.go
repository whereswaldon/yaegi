package interp

import (
	"path"
)

// Gta performs a global types analysis on the AST, registering types,
// variables and functions symbols at package level, prior to CFG.
// All function bodies are skipped. GTA is necessary to handle out of
// order declarations and multiple source files packages.
func (interp *Interpreter) Gta(root *Node) {
	var pkgName string
	scope := interp.universe

	root.Walk(func(n *Node) bool {
		switch n.kind {
		case Define:
			varName := n.child[0].ident
			scope.sym[varName] = &Symbol{kind: Var, global: true, index: scope.inc(interp)}
			if len(n.child) > 1 {
				scope.sym[varName].typ = nodeType(interp, scope, n.child[1])
			} else {
				scope.sym[varName].typ = nodeType(interp, scope, n.anc.child[0].child[1])
			}
			return false

		case File:
			pkgName = n.child[0].ident
			if _, ok := interp.scope[pkgName]; !ok {
				interp.scope[pkgName] = scope.push(0)
			}
			scope = interp.scope[pkgName]

		case FuncDecl:
			n.typ = nodeType(interp, scope, n.child[2])
			scope.sym[n.child[1].ident] = &Symbol{kind: Func, typ: n.typ, node: n}
			if len(n.child[0].child) > 0 {
				// function is a method, add it to the related type
				var receiverType *Type
				var typeName string
				n.ident = n.child[1].ident
				receiver := n.child[0].child[0]
				if len(receiver.child) < 2 {
					// Receiver var name is skipped in method declaration (fix that in AST ?)
					typeName = receiver.child[0].ident
				} else {
					typeName = receiver.child[1].ident
				}
				if typeName == "" {
					// The receiver is a pointer, retrieve typeName from indirection
					typeName = receiver.child[1].child[0].ident
					elemtype := scope.getType(typeName)
					if elemtype == nil {
						// Add type if necessary, so method can be registered
						scope.sym[typeName] = &Symbol{kind: Typ, typ: &Type{}}
						elemtype = scope.sym[typeName].typ
					}
					receiverType = &Type{cat: PtrT, val: elemtype}
					elemtype.method = append(elemtype.method, n)
				} else {
					receiverType = scope.getType(typeName)
					if receiverType == nil {
						// Add type if necessary, so method can be registered
						scope.sym[typeName] = &Symbol{kind: Typ, typ: &Type{}}
						receiverType = scope.sym[typeName].typ
					}
				}
				receiverType.method = append(receiverType.method, n)
			}
			return false

		case ImportSpec:
			var name, ipath string
			if len(n.child) == 2 {
				ipath = n.child[1].val.(string)
				name = n.child[0].ident
			} else {
				ipath = n.child[0].val.(string)
				name = path.Base(ipath)
			}
			if pkg, ok := interp.binValue[ipath]; ok {
				if name == "." {
					for n, s := range pkg {
						scope.sym[n] = &Symbol{typ: &Type{cat: BinT}, val: s}
					}
				} else {
					scope.sym[name] = &Symbol{typ: &Type{cat: BinPkgT}, path: ipath}
				}
			} else {
				// TODO: make sure we do not import a src package more than once
				interp.importSrcFile(ipath)
				scope.sym[name] = &Symbol{typ: &Type{cat: SrcPkgT}, path: ipath}
			}

		case TypeSpec:
			typeName := n.child[0].ident
			if n.child[1].kind == Ident {
				n.typ = &Type{cat: AliasT, val: nodeType(interp, scope, n.child[1])}
			} else {
				n.typ = nodeType(interp, scope, n.child[1])
			}
			// Type may already be declared for a receiver in a method function
			if scope.sym[typeName] == nil {
				scope.sym[typeName] = &Symbol{kind: Typ}
			} else {
				n.typ.method = append(n.typ.method, scope.sym[typeName].typ.method...)
			}
			scope.sym[typeName].typ = n.typ
			return false

		}
		return true
	}, nil)
}
