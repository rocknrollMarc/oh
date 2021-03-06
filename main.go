/*
Oh is a Unix shell.  It is similar in spirit but different in detail from
other Unix shells. The following commands behave as expected:

    date
    cat /usr/share/dict/words
    who >user.names
    who >>user.names
    wc <file
    echo [a-f]*.c
    who | wc
    who; date
    cc *.c &
    mkdir junk && cd junk
    cd ..
    rm -r junk || echo "rm failed!"

For more detail, see: https://github.com/michaelmacinnis/oh

Oh is released under an MIT-style license.
*/
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

var done0 chan Cell
var eval0 chan Cell

var jobs = map[int]*Task{}

func broker() {
	irq := Incoming()

	go NewForegroundTask().Listen()

	var c Cell
	for c == nil && ForegroundTask().Stack != Null {
		for c == nil {
			select {
			case <-irq:
			case c = <-eval0:
			}
		}
		ForegroundTask().Eval <- c
		for c != nil {
			prev := ForegroundTask()
			select {
			case sig := <-irq:
				// Handle signals.
				switch sig {
				case StopRequest:
					ForegroundTask().Suspend()
					last := 0
					for k := range jobs {
						if k > last {
							last = k
						}
					}
					last++

					jobs[last] = ForegroundTask()

					fallthrough
				case InterruptRequest:
					if sig == InterruptRequest {
						ForegroundTask().Stop()
					}
					fmt.Printf("\n")

					go NewForegroundTask().Listen()
					c = nil
				}

			case c = <-ForegroundTask().Done:
				if ForegroundTask() != prev {
					c = Null
					continue
				}
			}
		}
		done0 <- c
	}
	os.Exit(status(Car(ForegroundTask().Scratch)))
}

func evaluate(c Cell) {
	eval0 <- c
	<-done0

	task := ForegroundTask()
	task.Job.command = ""
	task.Job.group = 0
}

func init() {
	done0 = make(chan Cell)
	eval0 = make(chan Cell)
}

func main() {
	go broker()

	scope0 = RootScope()
	scope0.DefineBuiltin("fg", func(t *Task, args Cell) bool {
		if !JobControlEnabled() || t != ForegroundTask() {
			return false
		}

		index := 0
		if args != Null {
			if a, ok := Car(args).(Atom); ok {
				index = int(a.Int())
			}
		} else {
			for k := range jobs {
				if k > index {
					index = k
				}
			}
		}

		found, ok := jobs[index]

		if !ok {
			return false
		}

		if found.Job.group != 0 {
			SetForegroundGroup(found.Job.group)
			found.Job.mode.ApplyMode()
		}

		delete(jobs, index)

		SetForegroundTask(found)

		t.Stop()
		found.Continue()

		return true
	})
	scope0.DefineBuiltin("jobs", func(t *Task, args Cell) bool {
		if !JobControlEnabled() || t != ForegroundTask() ||
			len(jobs) == 0 {
			return false
		}

		i := make([]int, 0, len(jobs))
		for k := range jobs {
			i = append(i, k)
		}
		sort.Ints(i)
		for k, v := range i {
			if k != len(jobs)-1 {
				fmt.Printf("[%d] \t%d\t%s\n", v, jobs[v].Job.group, jobs[v].Job.command)
			} else {
				fmt.Printf("[%d]+\t%d\t%s\n", v, jobs[v].Job.group, jobs[v].Job.command)
			}
		}
		return false
	})

	Parse(nil, bufio.NewReader(strings.NewReader(`
define caar: method (l) as: car: car l
define cadr: method (l) as: car: cdr l
define cdar: method (l) as: cdr: car l
define cddr: method (l) as: cdr: cdr l
define caaar: method (l) as: car: caar l
define caadr: method (l) as: car: cadr l
define cadar: method (l) as: car: cdar l
define caddr: method (l) as: car: cddr l
define cdaar: method (l) as: cdr: caar l
define cdadr: method (l) as: cdr: cadr l
define cddar: method (l) as: cdr: cdar l
define cdddr: method (l) as: cdr: cddr l
define caaaar: method (l) as: caar: caar l
define caaadr: method (l) as: caar: cadr l
define caadar: method (l) as: caar: cdar l
define caaddr: method (l) as: caar: cddr l
define cadaar: method (l) as: cadr: caar l
define cadadr: method (l) as: cadr: cadr l
define caddar: method (l) as: cadr: cdar l
define cadddr: method (l) as: cadr: cddr l
define cdaaar: method (l) as: cdar: caar l
define cdaadr: method (l) as: cdar: cadr l
define cdadar: method (l) as: cdar: cdar l
define cdaddr: method (l) as: cdar: cddr l
define cddaar: method (l) as: cddr: caar l
define cddadr: method (l) as: cddr: cadr l
define cdddar: method (l) as: cddr: cdar l
define cddddr: method (l) as: cddr: cddr l
define $connect: syntax (type out close) as {
    set type: eval type
    set close: eval close
    syntax e (left right) as {
        define p: type
        spawn {
            eval: list 'dynamic out 'p
            e::eval left
            if close: p::writer-close
        }
	block {
            dynamic $stdin = p
            e::eval right
            if close: p::reader-close
	}
    }
}
define $redirect: syntax (chan mode mthd) as {
    syntax e (c cmd) as {
        define c: e::eval c
        define f = '()
        if (not: or (is-channel c) (is-pipe c)) {
            set f: open c mode
            set c = f
        }
        eval: list 'dynamic chan 'c
        e::eval cmd
        if (not: is-null f): eval: cons 'f mthd
    }
}
define and: syntax e (: lst) as {
    define r = false
    while (not: is-null: car lst) {
        set r: e::eval: car lst
        if (not r): return r
        set lst: cdr lst
    }
    return r
}
define append-stderr: $redirect $stderr "a" writer-close
define append-stdout: $redirect $stdout "a" writer-close
define apply: method (f: args) as: f @args
define backtick: syntax e (cmd) as {
    define p: pipe
    spawn {
        dynamic $stdout = p
        e::eval cmd
        p::writer-close
    }
    define r: cons '() '()
    define c = r
    define l: p::readline
    while l {
	set-cdr c: cons l '()
	set c: cdr c
        set l: p::readline
    }
    p::reader-close
    return: cdr r
}
define channel-stderr: $connect channel $stderr true
define channel-stdout: $connect channel $stdout true
define echo: builtin (: args) as: $stdout::write @(for args symbol)
define error: builtin (: args) as: $stderr::write @args
define for: method (l m) as {
    define r: cons '() '()
    define c = r
    while (not: is-null l) {
        set-cdr c: cons (m: car l) '()
        set c: cdr c
        set l: cdr l
    }
    return: cdr r
}
define glob: builtin (: args) as: return args
define import: syntax e (name) as {
    define m: module: e::eval name
    if (or (is-null m) (is-object m)) {
        return m
    }

    define l: list 'source name
    set l: cons 'object: cons l '()
    set l: list '$root::define m l
    e::eval l
}
define is-list: method (l) as {
    if (is-null l): return false
    if (not: is-cons l): return false
    if (is-null: cdr l): return true
    is-list: cdr l
}
define is-text: method (t) as: or (is-string t) (is-symbol t)
define object: syntax e (: body) as {
    e::eval: cons 'block: append body '(context)
}
define or: syntax e (: lst) as {
    define r = false
    while (not: is-null: car lst) {
	set r: e::eval: car lst
        if r: return r
        set lst: cdr lst 
    }
    return r
}
define substitute-stdin: syntax () as {
    echo "process substitution not yet implemented."
}
define substitute-stdout: syntax () as {
    echo "process substitution not yet implemented."
}
define pipe-stderr: $connect pipe $stderr true
define pipe-stdout: $connect pipe $stdout true
define printf: method (: args) as: echo: (car args)::sprintf @(cdr args)
define quote: syntax (cell) as: return cell
define read: builtin () as: $stdin::read
define readline: builtin () as: $stdin::readline
define redirect-stderr: $redirect $stderr "w" writer-close
define redirect-stdin: $redirect $stdin "r" reader-close
define redirect-stdout: $redirect $stdout "w" writer-close
define source: syntax e (name) as {
	define basename: e::eval name
	define paths = '()
	define name = basename

	if (has $OHPATH): set paths: (string $OHPATH)::split ":"
	while (and (not: is-null paths) (not: exists name)) {
		set name: "/"::join (car paths) basename
		set paths: cdr paths
	}

	if (not: exists name): set name = basename

        define r: cons '() '()
        define c = r
	define f: open name "r-"
	define l: f::read
	while l {
		set-cdr c: cons l '()
		set c: cdr c
		set l: f::read
	}
	set c: cdr r
	f::close
	define eval-list: method (rval first rest) as {
		if (is-null first): return rval
		eval-list (e::eval first) (car rest) (cdr rest)
	}
	eval-list (status 0) (car c) (cdr c)
}
define write: method (: args) as: $stdout::write @args
define list-ref: method (k x) as: car: list-tail k x
define list-tail: method (k x) as {
    if k {
        list-tail (sub k 1): cdr x
    } else {
        return x
    }
}

exists ("/"::join $HOME .ohrc) && source ("/"::join $HOME .ohrc)
`)), evaluate)

	if Interactive() {
		cli := Interface()

		Parse(nil, cli, evaluate)

		cli.Close()
		fmt.Printf("\n")
	} else if len(os.Args) > 1 {
		evaluate(List(NewSymbol("source"), NewSymbol(os.Args[1])))
	} else {
		evaluate(List(NewSymbol("source"), NewSymbol("/dev/stdin")))
	}

	os.Exit(0)
}
