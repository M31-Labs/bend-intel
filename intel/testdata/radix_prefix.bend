type MyMap:
  Free
  Used
  Both { ~a: MyMap, ~b: MyMap }

type Arr(t):
  Null
  Leaf { a: t }
  Node { ~a: Arr(t), ~b: Arr(t) }

def swap(s: u24, a: MyMap, b: MyMap) -> MyMap:
  switch s:
    case 0:
      return MyMap/Both{ a: a, b: b }
    case _:
      return MyMap/Both{ a: b, b: a }


def sort(t: Arr(u24)) -> Arr(u24):
  return to_arr(0, to_map(t))

def to_map(t: Arr(u24)) -> MyMap:
  match t:
    case Arr/Null:
      return MyMap/Free
    case Arr/Leaf:
      return radix(t.a)
    case Arr/Node:
      return merge(to_map(t.a), to_map(t.b))

def to_arr(x: u24, m: MyMap) -> Arr(u24):
  match m:
    case MyMap/Free:
      return Arr/Null
    case MyMap/Used:
      return Arr/Leaf{ a: x }
    case MyMap/Both:
      return Arr/Node{ a: to_arr(x * 2, m.a), b: to_arr(x * 2 + 1, m.b) }

def merge(a: MyMap, b: MyMap) -> MyMap:
  match a:
    case MyMap/Free:
      return b
    case MyMap/Used:
