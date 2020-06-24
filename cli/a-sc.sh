#!/bin/bash
#
# Author: Sebastian Robitzsch <sebastian.robitzsch@interdigital.com>
#
# Implementation of the A_{SC} RESTful ARDENT API

aScHelp()
{
        if [ $# -eq 0 ]
        then
                echo -e "\tardent check"
        else
                echo -e "\tardent check run [--wait]"
                echo -e "\tardent check status"
                echo -e "\tardent check results"
        fi
}

aScCheck()
{
        case "$1" in
        "run")
        	echo -e "---\nRequesting sanity check\n---"
                aScRun $2 
                ;;
        "status")
        	echo -e "---\nRequesting status\n---"
                aScStatus
                ;;
	"results")
        	echo -e "---\nGetting results\n---"
		aScResults
		;;
        *)
                echo "Unknown argument '$1'"
                ;;
        esac
}

###############################################################################

aScRun()
{
	http POST http://$ARDENT:$ARDENT_PORT/sanity-check

	if [ $# -eq 1 ]
	then
		case "$1" in
			"--wait")
				echo -en "---\nWaiting to complete"

				# wait until it has completed
				while aScStatus | grep "in progress" > /dev/null
				do
					echo -n "."
					sleep 1
				done
				
				echo -e "\n---"
				aScStatus
				;;
			*)
				echo "Unknown argument $1"
				;;
		esac
	fi
}

aScStatus()
{
	http GET http://$ARDENT:$ARDENT_PORT/sanity-check/status
}

aScResults()
{
        http GET http://$ARDENT:$ARDENT_PORT/sanity-check/results
}

