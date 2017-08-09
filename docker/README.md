Docker image of ht
==================

This folder provides a tiny docker image containing ht.


Usage of the image
------------------

To display version information, help, or builtin documentation use:

    $ docker run ht version
    $ docker run ht help
    $ docker run ht help help
    $ docker run ht help exec
    $ docker run doc StatusCode

To execute test, suites, load tests etc, provide a volumen under `/wd`
which is the workdir for ht.

    docker run -v $(pwd):/wd ht list showcase.suite

