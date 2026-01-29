# Introduction

This is a **directory**picker bubble, a bubble being a component for
Bubble Tea applications. It was inspired by the [official filepicker
Bubble](https://github.com/charmbracelet/bubbles/tree/master/filepicker) — much of the code is taken directly from that example —
but pared down to the simplicity of the [shopping list tutorial](https://github.com/charmbracelet/bubbletea?tab=readme-ov-file#tutorial)
found in the Bubble Tea README.

# Motivation

I needed something customizable and lightweight for my Gogent [REPL
frontend](https://github.com/BrandonIrizarry/gogent_repl). It's indeed possible to use the official filepicker for
selecting directories, but visually unintuitive since you must have
already entered a directory to select it. So I had the idea of
introducing a checkbox-selection mechanism:

**OK**

```
/home/me/
      CoolProject/
      ↪ I want this directory
      
      AwesomeProject/
```

**Better**

```
/home/me/
     [x] CoolProject/ → I want this directory
     [ ] AwesomeProject/
```

This also enables the selection of multiple directories, which are
accessible as a slice of strings contained in the model returned by
`Run()`. This feature can be useful in the case where an LLM might
need access to multiple working directories (for example, to compare
two projects for similarities and differences.)

# Some Terminology

Throughout the codebase, I use `point` — terminology borrowed from
Emacs — to refer to where the UI cursor is currently resting, to avoid
the word "selection", since that refers to actively commiting the
entry at point to the model state.
