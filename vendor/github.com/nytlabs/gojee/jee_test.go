package jee

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
)

type Test struct {
	exp    string
	result string
}

var Tests = []Test{
	{
		exp:    `.int == 5`,
		result: `true`,
	},
	{
		exp:    `.float == 5.5`,
		result: `true`,
	},
	{
		exp:    `.["es'c\"ape'.key"]`,
		result: `null`,
	},
	{
		exp:    `.['escape.key']`,
		result: `{"nested":{"foo.bar":"baz"}}`,
	},
	{
		exp:    `.['escape.key']['nested']`,
		result: `{"foo.bar":"baz"}`,
	},
	{
		exp:    `.['escape.key']['nested']['foo.bar']`,
		result: `"baz"`,
	},
	{
		exp:    `.['escape.key'].nested["foo.bar"]`,
		result: `"baz"`,
	},
	{
		exp:    `.string == "hello world"`,
		result: `true`,
	},
	{
		exp:    `.int -- .int`,
		result: `10`,
	},
	{
		exp:    `.float - -.float`,
		result: `11`,
	},
	{
		exp:    `.int +-.float`,
		result: `-0.5`,
	},
	{
		exp:    `.int/-.float`,
		result: `-0.9090909090909091`,
	},
	{
		exp:    `-.float/-.float`,
		result: `1`,
	},
	{
		exp:    `-.float/.float`,
		result: `-1`,
	},
	{
		exp:    `.bool == false`,
		result: `true`,
	},
	{
		exp:    `.nil == null`,
		result: `true`,
	},
	{
		exp:    `null == .nil`,
		result: `true`,
	},
	{
		exp:    `.nested.foo.zip`,
		result: `"zap"`,
	},
	{
		exp:    `.arrayInt`,
		result: `[1,2,3,4,5,6,7,8,9,10]`,
	},
	{
		exp:    `.arrayFloat`,
		result: `[1.1,2.2,3.3,4.4,5.5,6.6,7.7,8.8,9.9,10]`,
	},
	{
		exp:    `.arrayInt[0]`,
		result: `1`,
	},
	{
		exp:    `.arrayObj[0].nested[0].id`,
		result: `"foo"`,
	},
	{
		exp:    `.arrayInt[]`,
		result: `[1,2,3,4,5,6,7,8,9,10]`,
	},
	{
		exp:    `.arrayObj[]`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.arrayObj`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.arrayObj[].name`,
		result: `["foo","bar","baz"]`,
	},
	{
		exp:    `.arrayObj[].val`,
		result: `[2,2.5,10]`,
	},
	{
		exp:    `.arrayObj[].array[]`,
		result: `[1,2,3,1,2,3,7,8,9]`,
	},
	{
		exp:    `.arrayObj[].array`,
		result: `[[1,2,3],[1,2,3],[7,8,9]]`,
	},
	{
		exp:    `.arrayObj[0].array`,
		result: `[1,2,3]`,
	},
	{
		exp:    `(true && false) && true == false`,
		result: `true`,
	},
	{
		exp:    `$sum(.arrayInt)`,
		result: `55`,
	},
	{
		exp:    `$sum(.arrayInt) + $sum(.arrayInt[])`,
		result: `110`,
	},
	{
		exp:    `$sum(.arrayFloat) < 100.0`,
		result: `true`,
	},
	{
		exp:    `$sum(.arrayObj[].array[]) == 36`,
		result: `true`,
	},
	{
		exp:    `true && true && false || (!true) == !true`,
		result: `true`,
	},
	{
		exp:    `true && false && (true && (true && false))`,
		result: `false`,
	},
	{
		exp:    `!(.int == 2 + 3) == false`,
		result: `true`,
	},
	{
		exp:    `100 - ((3/2)*20 + 7 -8)`,
		result: `71`,
	},
	{
		exp:    `     100.0 -          ( (   3/2 )*20 + 7 -8  )`,
		result: `71`,
	},
	{
		exp:    `"hello" + " " + "world"`,
		result: `"hello world"`,
	},
	{
		exp:    `true && true && (true || false)`,
		result: `true`,
	},
	{
		exp:    `(true || false) && true && true`,
		result: `true`,
	},
	{
		exp:    `false`,
		result: `false`,
	},
	{
		exp:    `true`,
		result: `true`,
	},
	{
		exp:    `null`,
		result: `null`,
	},
	{
		exp:    `.['escape.key']`,
		result: `{"nested":{"foo.bar":"baz"}}`,
	},
	{
		exp:    `.arrayInt[0]`,
		result: `1`,
	},
	{
		exp:    `.['arrayInt'][0]`,
		result: `1`,
	},
	{
		exp:    `.arrayObj[0].nested`,
		result: `[{"id":"foo","no":"zoo"}]`,
	},
	{
		exp:    `.['arrayObj'][0]['nested']`,
		result: `[{"id":"foo","no":"zoo"}]`,
	},
	{
		exp:    `.arrayObj[]`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.['arrayObj'][]['nested'][]['id']`,
		result: `["foo","zof","zif"]`,
	},
	{
		exp:    `.arrayObj[].array[2]`,
		result: `[3,3,9]`,
	},
	{
		exp:    `.arrayObj[1].nested[].id`,
		result: `["zof"]`,
	},
	{
		exp:    `.arrayFloat`,
		result: `[1.1,2.2,3.3,4.4,5.5,6.6,7.7,8.8,9.9,10]`,
	},
	{
		exp:    `.arrayFloat[]`,
		result: `[1.1,2.2,3.3,4.4,5.5,6.6,7.7,8.8,9.9,10]`,
	},
	{
		exp:    `.arrayObj`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.arrayObj[]`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.arrayObj[].array`,
		result: `[[1,2,3],[1,2,3],[7,8,9]]`,
	},
	{
		exp:    `.arrayObj[].array[]`,
		result: `[1,2,3,1,2,3,7,8,9]`,
	},
	{
		exp:    `.arrayObj[].array[1]`,
		result: `[2,2,8]`,
	},
	{
		exp:    `.arrayObj[].nested[].id`,
		result: `["foo","zof","zif"]`,
	},
	{
		exp:    `.arrayObj[0].nested[0].id`,
		result: `"foo"`,
	},
	{
		exp:    `.arrayObj[2].nested[].id`,
		result: `["zif"]`,
	},
	{
		exp:    `.['arrayFloat']`,
		result: `[1.1,2.2,3.3,4.4,5.5,6.6,7.7,8.8,9.9,10]`,
	},
	{
		exp:    `.['arrayFloat'][]`,
		result: `[1.1,2.2,3.3,4.4,5.5,6.6,7.7,8.8,9.9,10]`,
	},
	{
		exp:    `.['arrayObj']`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.['arrayObj'][]`,
		result: `[{"array":[1,2,3],"bool":false,"hasKey":true,"name":"foo","nested":[{"id":"foo","no":"zoo"}],"nil":null,"sameNum":10,"sameStr":"all","val":2},{"array":[1,2,3],"bool":true,"name":"bar","nested":[{"id":"zof","no":"fum"}],"nil":null,"sameNum":10,"sameStr":"all","val":2.5},{"array":[7,8,9],"bool":false,"name":"baz","nested":[{"id":"zif","no":"zaf"}],"nil":null,"sameNum":10,"sameStr":"all","val":10}]`,
	},
	{
		exp:    `.['arrayObj'][]['array']`,
		result: `[[1,2,3],[1,2,3],[7,8,9]]`,
	},
	{
		exp:    `.['arrayObj'][]['array'][]`,
		result: `[1,2,3,1,2,3,7,8,9]`,
	},
	{
		exp:    `.['arrayObj'][]['array'][1]`,
		result: `[2,2,8]`,
	},
	{
		exp:    `.['arrayObj'][]['nested']`,
		result: `[[{"id":"foo","no":"zoo"}],[{"id":"zof","no":"fum"}],[{"id":"zif","no":"zaf"}]]`,
	},
	{
		exp:    `.['arrayObj'][]['nested'][]`,
		result: `[{"id":"foo","no":"zoo"},{"id":"zof","no":"fum"},{"id":"zif","no":"zaf"}]`,
	},
	{
		exp:    `.['arrayObj'][]['nested'][]['id']`,
		result: `["foo","zof","zif"]`,
	},
	{
		exp:    `.['arrayObj'][0]['nested'][0]['id']`,
		result: `"foo"`,
	},
	{
		exp:    `.['arrayObj'][2]['nested'][]['id']`,
		result: `["zif"]`,
	},
	{
		exp:    `$len(.arrayObj)`,
		result: `3`,
	},
	{
		exp:    `$len(.arrayObj[])`,
		result: `3`,
	},
	{
		exp:    `$len(.['arrayObj'][]['array'])`,
		result: `3`,
	},
	{
		exp:    `$len(.['arrayObj'][]['array'][])`,
		result: `9`,
	},
	{
		exp:    `-(2 * (-2 * (-5 + (-5))))`,
		result: `-40`,
	},
	{
		exp:    `$pow(.int,2)`,
		result: `25`,
	},
	{
		exp:    `$sqrt(100)`,
		result: `10`,
	},
	{
		exp:    `$sqrt($sum(.arrayInt))`,
		result: `7.416198487095663`,
	},
	{
		exp:    `$pow( (-0.1) * 10, 2)`,
		result: `1`,
	},
	{
		exp:    `$abs(-100)`,
		result: `100`,
	},
	{
		exp:    `$abs(100)`,
		result: `100`,
	},
	{
		exp:    `$max(.arrayInt)`,
		result: `10`,
	},
	{
		exp:    `$min(.arrayInt)`,
		result: `1`,
	},
	{
		exp:    `$min(.arrayObj[].array[])`,
		result: `1`,
	},
	{
		exp:    `$floor(.arrayFloat[0])`,
		result: `1`,
	},
	{
		exp:    `$contains(.string,"hello")`,
		result: `true`,
	},
	{
		exp:    `$contains("http://en.wikipedia.org/wiki/List_of_animals_with_fraudulent_diplomas","wikipedia")`,
		result: `true`,
	},
	{
		exp:    `$contains("http://en.wikipedia.org/wiki/List_of_animals_with_fraudulent_diplomas","dogs")`,
		result: `false`,
	},
	// needs a special test ... $keys returns an unordered list
	//Test{
	//            exp:`$keys(.arrayObj[0])`,
	//            result:`["val","name","sameStr","hasKey","nil","array","nested","bool","sameNum"]`,
	//},
	{
		exp:    `$has($keys(.), "arrayString")`,
		result: `true`,
	},
	{
		exp:    `$has($keys(.), "nope")`,
		result: `false`,
	},
	{
		exp:    `$exists(., "arrayString")`,
		result: `true`,
	},
	{
		exp:    `$exists(., "nope")`,
		result: `false`,
	},
	{
		exp:    `$has(.arrayFloat, 1.1)`,
		result: `true`,
	},
	{
		exp:    `$has($keys(.), "arrayString") || $has($keys(.), "nope") `,
		result: `true`,
	},
	{
		exp:    `$has($keys(.), "arrayString") && $has($keys(.), "nope") `,
		result: `false`,
	},
	{
		exp:    `.#_k__`,
		result: `1`,
	},
	{
		exp:    `$num(.float_str) == 5.123131`,
		result: `true`,
	},
	{
		exp:    `$str($num(.float_str)) == .float_str`,
		result: `true`,
	},
	{
		exp:    `$parseTime("Mon Jan 2 15:04:05 -0700 MST 2006","Wed Jan 1 00:00:00 +0000 GMT 2014") == 1388534400000`,
		result: `true`,
	},
	{
		exp:    `$num($fmtTime("2006", $parseTime("Mon Jan 2 15:04:05 -0700 MST 2006","Wed Jan 1 00:00:00 +0000 GMT 2014"))) == 2014`,
		result: `true`,
	},
	{
		exp:    `$now() > 1388534400000`,
		result: `true`,
	},
	{
		exp:    `$num($fmtTime("2006", $now())) > 2006`,
		result: `true`,
	},
	{
		exp:    `$str(.float_str) == "5.123131"`,
		result: `true`,
	},
	{
		exp:    `$num(.int) == 5`,
		result: `true`,
	},
	{
		exp:    `$num(.bool) == 0`,
		result: `true`,
	},
	{
		exp:    `$num(.a) == 0`,
		result: `true`,
	},
	{
		exp:    `$num(.empty) == 0`,
		result: `true`,
	},
	{
		exp:    `$bool("true") && true`,
		result: `true`,
	},
	{
		exp:    `$bool("false") && true`,
		result: `false`,
	},
	{
		exp:    `$bool(1)`,
		result: `null`,
	},
	{
		exp:    `$bool(null)`,
		result: `null`,
	},
	{
		exp:    `$~bool(null)`,
		result: `false`,
	},
	{
		exp:    `$~bool(.empty)`,
		result: `false`,
	},
	{
		exp:    `$~bool(.a.b.c)`,
		result: `true`,
	},
	{
		exp:    `$~bool("asdsajdasd")`,
		result: `true`,
	},
	{
		exp:    `$~bool(1)`,
		result: `true`,
	},
}

func TestAll(t *testing.T) {
	var umsg BMsg

	testFile, _ := ioutil.ReadFile("test.json")

	json.Unmarshal(testFile, &umsg)

	for _, test := range Tests {
		tokenized, err := Lexer(test.exp)
		if err != nil {
			t.Error("failed lex")
		}

		tree, err := Parser(tokenized)
		if err != nil {
			t.Error("failed parse")
		}

		result, err := Eval(tree, umsg)

		if err != nil {
			t.Error("failed eval")
		}

		var rmsg BMsg
		err = json.Unmarshal([]byte(test.result), &rmsg)
		if err != nil {
			t.Error(err, "bad test")
		}

		if reflect.DeepEqual(rmsg, result) {
			fmt.Println("\x1b[32;1mOK\x1b[0m", test.exp)
		} else {
			t.Fail()
			fmt.Println("\x1b[31;1m", "FAIL", "\x1b[0m", rmsg, "\t", result)
			fmt.Println("Expected Value", rmsg, "\tResult Value:", result)
			fmt.Println("Expected Type: ", reflect.TypeOf(rmsg), "\tResult Type:", reflect.TypeOf(result))
		}
	}
}

func BenchmarkJSON(b *testing.B) {
	var umsg BMsg
	testFile, _ := ioutil.ReadFile("test.json")
	json.Unmarshal(testFile, &umsg)
	tokenized, _ := Lexer(`.['arrayObj'][2]['nested'][]['id']`)
	tree, _ := Parser(tokenized)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Eval(tree, umsg)
	}
}

func BenchmarkMath(b *testing.B) {
	var umsg BMsg
	testFile, _ := ioutil.ReadFile("test.json")
	json.Unmarshal(testFile, &umsg)
	tokenized, _ := Lexer(`100 * -($sum(.arrayInt) + 5)`)
	tree, _ := Parser(tokenized)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Eval(tree, umsg)
	}
}

func BenchmarkRegex(b *testing.B) {
	var umsg BMsg
	testFile, _ := ioutil.ReadFile("test.json")
	json.Unmarshal(testFile, &umsg)
	tokenized, _ := Lexer(`$regex(.string, "hello*")`)
	tree, _ := Parser(tokenized)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Eval(tree, umsg)
	}
}

func BenchmarkContains(b *testing.B) {
	var umsg BMsg
	testFile, _ := ioutil.ReadFile("test.json")
	json.Unmarshal(testFile, &umsg)
	tokenized, _ := Lexer(`$contains(.string, "hello")`)
	tree, _ := Parser(tokenized)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Eval(tree, umsg)
	}
}
