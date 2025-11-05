#!/bin/bash
# Echo Agent Script - Runs inside the container
# Reads from stdin, processes, and writes to stdout

echo "Echo Agent started. Send me messages!"
echo "---"

while IFS= read -r line; do
    # Check for exit command
    if [ "$line" = "exit" ] || [ "$line" = "quit" ]; then
        echo "Echo Agent shutting down. Goodbye!"
        exit 0
    fi

    # Echo the received message
    echo "Received: $line"

    # Process the message (convert to uppercase)
    processed=$(echo "$line" | tr '[:lower:]' '[:upper:]')
    echo "Processed: $processed"

    # Calculate length
    length=${#line}
    echo "Length: $length characters"

    echo "---"
done

# If stdin closes, exit gracefully
echo "Echo Agent: stdin closed, exiting."
