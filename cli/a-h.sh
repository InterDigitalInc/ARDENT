#!/bin/bash
#
# Author: Sebastian Robitzsch <sebastian.robitzsch@interdigital.com>
#
# Implementation of the A_{H} RESTful ARDENT API

###############################################################################
aHHelp()
{
        if [ $# -eq 0 ]
        then
                echo -e "\tardent hot"
        else
                echo -e "\tardent hot generate"
                echo -e "\tardent hot show"
                echo -e "\tardent hot delete"
        fi
}

aHHot()
{
        case "$1" in
        "generate")
		echo -e "---\nGenerating HOT\n---"
                aHGenerate
                ;;
        "show")
		echo -e "---\nShowing HOT\n---"
                aHShow
                ;;
        "delete")
                aHDelete
                ;;
        *)
                echo "Unknown argument '$1'"
                ;;
        esac
}

###############################################################################
aHGenerate()
{
	http POST http://$ARDENT:$ARDENT_PORT/hot/generate
}

aHShow()
{
	http GET http://$ARDENT:$ARDENT_PORT/hot/descriptor
}

aHDelete()
{
	http DELETE http://$ARDENT:$ARDENT_PORT/hot/descriptor
}

