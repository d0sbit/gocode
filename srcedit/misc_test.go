package srcedit

import "fmt"

func ExampleLowerForType() {

	fmt.Println(LowerForType("Something", "-"))
	fmt.Println(LowerForType("SomeThing", "-"))
	fmt.Println(LowerForType("HTTPSomething", "-"))
	fmt.Println(LowerForType("YetAnotherThing", "_"))

	// Output:
	// something
	// some-thing
	// http-something
	// yet_another_thing

}
