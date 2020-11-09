package jsoniter

// ReadArray read array element, tells if the array has more element to read.
func (iter *Iterator) ReadArray() (ret bool) {
	c := iter.nextToken()
	switch c {
	case 'n':
		iter.skipThreeBytes('u', 'l', 'l')
		return false // null
	case '[':
		c = iter.nextToken()
		if c != ']' {
			iter.unreadByte()
			return true
		}
		return false
	case ']':
		return false
	case ',':
		return true
	default:
		iter.ReportError("ReadArray", "expect [ or , or ] or n, but found "+string([]byte{c}))
		return
	}
}

// ReadArrayCB read array with callback
func (iter *Iterator) ReadArrayCB(callback func(*Iterator) bool) (ret bool) {
	c := iter.nextToken()
	if c == '[' {
<<<<<<< HEAD
		if !iter.incrementDepth() {
			return false
		}
=======
>>>>>>> cbc9bb05... fixup add vendor back
		c = iter.nextToken()
		if c != ']' {
			iter.unreadByte()
			if !callback(iter) {
<<<<<<< HEAD
				iter.decrementDepth()
=======
>>>>>>> cbc9bb05... fixup add vendor back
				return false
			}
			c = iter.nextToken()
			for c == ',' {
				if !callback(iter) {
<<<<<<< HEAD
					iter.decrementDepth()
=======
>>>>>>> cbc9bb05... fixup add vendor back
					return false
				}
				c = iter.nextToken()
			}
			if c != ']' {
				iter.ReportError("ReadArrayCB", "expect ] in the end, but found "+string([]byte{c}))
<<<<<<< HEAD
				iter.decrementDepth()
				return false
			}
			return iter.decrementDepth()
		}
		return iter.decrementDepth()
=======
				return false
			}
			return true
		}
		return true
>>>>>>> cbc9bb05... fixup add vendor back
	}
	if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		return true // null
	}
	iter.ReportError("ReadArrayCB", "expect [ or n, but found "+string([]byte{c}))
	return false
}
