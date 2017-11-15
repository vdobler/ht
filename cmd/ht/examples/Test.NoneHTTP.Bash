// Executing bash scripts
{
    Name: "bash pseudo-requests"

    Description: '''
        It is possible to have a Test execute a Bash script instead of making
        a HTTP request. The checks (and extractions) are applied to the
        output of the script.
    '''

    Request: {
        // The 'bash://' schema makes this a Bash-pseudo-request.
        // Bash scripts can be executed only on the same machine (localhost).
        // The Working Directory in which the script is executed is the path
        // of the URL. So you cannot use relative paths.
        URL:    "bash://localhost/{{CWD}}/{{TEST_DIR}}"

        // The request params are used as environment in which the script
        // executes: Inside the script the bash variables ENVVAR1 and ENVVAR2
        // will be set with the given values.
        Params: {
            ENVVAR1: "some value"
            ENVVAR2: "12345"
        }

        // The script's running time is limited to Timeout
	Timeout: "25s"

        // The body contains the bash script. This is just an example.
        // The body could be read from a file with the @[v]file-syntax:
        //     Body: "@vfile:{{TEST_DIR}}/script.bash"
        // For this simple example we provide it directly.
        Body: '''
            # Run the ls command in long form
	    ls -l 
        '''

        // All other fields of Request are ignored for bash pseudo-requests.
    }

    Checks: [
        // Bash pseudo-request report in the  HTTP status code:
        //    - 200 if the script's exit code is 0.
        //    - 408 if the script was canceled due to timeout
        //    - 500 if the exit code is != 0.
        {Check: "StatusCode", Expect: 200}

        // The numerical exits status itself is reported in the response
        // header Exit-Status:
        {Check: "Header", Header: "Exit-Status", Equals: "exit status 0"}

        // The combined output (stdout and stderr) of the script is returned
        // as the response body:
        {Check: "Body", Contains: "Suite.Variables"}
    ]
}
