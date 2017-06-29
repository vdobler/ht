Showcase for cmd/ht
===================

This folder contains a pretty much nonsensical showcase suite of a lot
of features doable with ht.

Start a small demo server and a MySQL docker instance before executing
the showcase suite:

    $ go run showcase.go &
    $ docker run --rm -d -e MYSQL_USER=test -e MYSQL_PASSWORD=test -e MYSQL_DATABASE=test -e MYSQL_ALLOW_EMPTY_PASSWORD=true -p 7799:3306 mysql:5.6

    $ ht exec -output result showcase.suite

This should produce 