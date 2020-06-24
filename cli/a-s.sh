#!/bin/bash
#
# Author: Sebastian Robitzsch <sebastian.robitzsch@interdigital.com>
#
# Implementation of the A_{DS} RESTful ARDENT API

###############################################################################

aSHelp()
{
	if [ $# -eq 0 ]
        then
                echo -e "\tardent stack"
        else
                echo -e "\tardent stack create [--wait]"
                echo -e "\tardent stack delete [--wait]"
                echo -e "\tardent stack status"
	fi
}

aSStack()
{
	case "$1" in
		"create")
			echo -e "---\nCreating stack '$ARDENT_STACK_NAME'\n---"
			aSCreate $2
			;;
		"delete")
			echo -e "---\nDeleting stack '$ARDENT_STACK_NAME'\n---"
			aSDelete $2
			;;
		"status")
			echo -e "---\nReading stack status\n---"
			aSStatus
			;;
		*)
			echo "Unknown argument $2"
			aSHelp
			;;
	esac
}

###############################################################################

aSCreate()
{
	http POST http://$ARDENT:$ARDENT_PORT/stack/create name=$ARDENT_STACK_NAME

	if [ $# -gt 0 ]
	then
		if [ $1 == "--wait" ]
		then
			echo -en "---\nWaiting for stack creation to complete"

			while aSStatus | grep "in progress" > /dev/null
			do
				echo -n "."
			done

			echo -e "\n---"
			aSStatus
		else
			echo "Unknown argument '$1'"
		fi
	fi
}

aSDelete()
{
	http POST http://$ARDENT:$ARDENT_PORT/stack/delete name=$ARDENT_STACK_NAME

        if [ $# -gt 0 ]
        then
                if [ $1 == "--wait" ]
                then
                        echo -en "---\nWaiting for stack deletion to complete"

                        while aSStatus | grep "in progress" > /dev/null
                        do
                                echo -n "."
                        done

                        echo -e "\n---"
                        aSStatus
                else
                        echo "Unknown argument '$1'"
                fi
        fi

}

aSStatus()
{
	http http://$ARDENT:$ARDENT_PORT/stack/status/$ARDENT_STACK_NAME
}

