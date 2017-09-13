Docker image of ht
==================

This folder provides a tiny docker image containing ht.

The image contains basically only a staticaly linked version of ht and
TLS certificates to verify https connections. It is thus very small.


Usage of the image
------------------

To display version information, the help, or builtin documentation use:

    $ docker run --rm vodo/ht version
    $ docker run --rm vodo/ht help
    $ docker run --rm vodo/ht help help
    $ docker run --rm vodo/ht help exec
    $ docker run --rm vodo/ht doc StatusCode

To execute test, suites, load tests etc, provide a volumen under `/app`
which is the workdir for ht.

    $ docker run --rm --volume $(pwd):/app vodo/ht list showcase.suite

The image does not contain any users, thus ht runs as root and output files
written by ht might be unreadable to you local host user. You should thus run
ht as your host user like this:

    $ docker run --rm --user $(id -u):$(id -g) -v $(pwd):/app vodo/ht exec showcase.suite

