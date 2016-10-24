Bugs and Design Errors
======================

Variable Substitution
---------------------

### Useless for the library

There are two ways to use ht:
 - as a library: `import "github.com/vdobler/ht/ht", and
 - as a command line programm `ht`.

The "library use" does not profit much from variable substututions as
it is much easier to construct a Test by some programming than by variable
substitutions (which are too simple). The special variabels like NOW
and RANDOM are simply too ugly to use if compared to code.

The "cmdline use" has no  way to script stuff, so it cruically depends
on variabel substitutions (and extractions).

Ht now mixes both uses up: It makes it easy to apply variables to
Checks (via reflection black magic) instead of exposing the variables in
the interface (which would not be too bothersome in the library use case).

Maybe it would be much saner to keep the textual representation
of Tests and Checks (the JSON5 files) during command line use and
instantiate Tests and Checks as needed by use of a better macro language:
The textual representation of a Test is read from disk during startup
and each time this Test has to be executed a real Test struct is
instantiated.  This instantiation can be a complex process with complex
macro-expansions (like \exapandafter). But ist is inclear how this could
solve the problem with making e.g. PreSleep configurable.


### Halfbacked substitutions

Variable substitution happens in two places:
 - During Unrolling a test (happens only during reading in from disk)
 - Before (each of the different tries during) execution.

Unrolling is ugly (least common multiple!). The feature is "needed" as it
isn't possible to "call" a test with a different set of variables like e.g.:

     Tests: [
         "homepage.ht" { "PROTOCOL": "http" },
         "homepage.ht" { "PROTOCOL": "https" },
         "search.ht",  { "QUERY": "Team", Hits: "Found 12 results." }, 
         "search.ht",  { "QUERY": "Blabla", Hits: "Sorry, no match found." },
         "signup.ht"   { "USER": "trude.customer@example.org" },
         "signup.ht"   { "USER": "someone.elser@{{DOMAIN}}" },
     ]

So:

Tests inside a suite should be:
    Tests: []struct{
        File string,
        Test: Test,
        Vars map[string]string,
    }

Like this one could directly include simple tests into the suite.
If File != "" it is treated as an implicite BasedOn for non-zero Test.



Suites and how to construct a Test from stuff on Disk
-----------------------------------------------------

The second of my major fuckups with ht.
githup.com/vdobler/ht/ht provides Test and different Checks, this is nice.
Combining them into a sequence of tests to be executed one after the
other is dead simple in code: What Suite does is:
 - provide a cookiejar
 - execute three for loops
 - copy extracted variables to the next test
Rocketsience at it finest.

Suite is useless brainfuck in githup.com/vdobler/ht/ht, it doesn't
belong there and could go into .../cmd/ht or a seperate package.

Like said above: Variable substitutions are usefull only for
stuff read from disk, not during library use of ht.


### TextTest as serialisation format

It would be sensible to have something like this:
 - A purely "textual" representation of a test including the checks
 - A way to subsutute variables in this textual repr.
 - A way to construct a ht.Test from this repr
 - A way to execute tests combined to a suite.

The purely texttual representation allows to subsitute variabels
_everywhere_ and the dead-stupid #98765-variables can be killed.

The sequence would be:
 
     Disk (hjson)
       |
       |  load
       V
     TextTest (working name, now rawTest)
       |
       |/--- merge Mixins
       V
     TextTest
       |
       |  substitute variables
       V
     TextTest
       |
       |  convert
       V
     Test (NotRun)
       |
       |  execute
       V
     Test (Pass, Fail, Error, ...)


Variable Substitution would work on some JSON soup which includes
the checks, no fanciness needed. Substitution could be made everywhere,
not just in selected fields.
TextTest would not contain stuff used only during execution or to
report the outcome.

Merging could be done like it is done now, just not on Test but on TextTest.

TextTest can life in its own package raw or pre (raw.Test vs. pre.Text)

Package ht does not need to know about raw (or pre), variable subst could
life completely in raw, Merging can life in raw. Converting to Test can
life in raw.


### Suites to execute them

Suite can go to it's own package suite. 

The suite would be just a selection of TextTests. During execution
each TextTest would be converted to a ht.Test which is executed.



