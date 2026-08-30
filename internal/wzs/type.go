package wzs

// Data type of a wz node.
//
// Go port of the Java enum provider.WzXML.MapleDataType (see
// docs/FILETRACK.md). The names are kept verbatim so call sites that mirror
// Java switch blocks stay readable; the Go prefix was added because the bare
// names (STRING, INT, ...) would collide with the usual Go vocabulary.

// DataType identifies the payload of a Node.
type DataType uint8

const (
	TypeNone DataType = iota // NONE
	TypeImg0x00              // null node (Java IMG_0x00)
	TypeShort
	TypeInt
	TypeLong
	TypeFloat
	TypeDouble
	TypeString
	TypeExtended
	TypeProperty // imgdir
	TypeCanvas
	TypeVector
	TypeConvex
	TypeSound
	TypeUOL
	TypeUnknownType
	TypeUnknownExtendedType
)

var dataTypeNames = [...]string{
	"NONE",
	"IMG_0x00",
	"SHORT",
	"INT",
	"LONG",
	"FLOAT",
	"DOUBLE",
	"STRING",
	"EXTENDED",
	"PROPERTY",
	"CANVAS",
	"VECTOR",
	"CONVEX",
	"SOUND",
	"UOL",
	"UNKNOWN_TYPE",
	"UNKNOWN_EXTENDED_TYPE",
}

// String returns the Java enum name of the type.
func (t DataType) String() string {
	if int(t) < len(dataTypeNames) {
		return dataTypeNames[t]
	}
	return "UNKNOWN_TYPE"
}

// dataTypeOf maps an XML element name to a DataType.
//
// Mirrors provider.WzXML.XMLDomMapleData.getType(): unknown tags map to
// UNKNOWN_TYPE (the Java version returned null, which blew up later with an
// NPE; Go keeps the node and lets callers decide).
func dataTypeOf(tag string) DataType {
	switch tag {
	case "imgdir":
		return TypeProperty
	case "canvas":
		return TypeCanvas
	case "convex":
		return TypeConvex
	case "sound":
		return TypeSound
	case "uol":
		return TypeUOL
	case "double":
		return TypeDouble
	case "float":
		return TypeFloat
	case "int":
		return TypeInt
	case "long":
		return TypeLong
	case "short":
		return TypeShort
	case "string":
		return TypeString
	case "vector":
		return TypeVector
	case "null":
		return TypeImg0x00
	}
	return TypeUnknownType
}
