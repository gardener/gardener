#!/bin/bash

# use injected image as sandbox image
sandbox_image_line="$(grep sandbox_image $FILE | sed -e 's/^[ ]*//')"
pause_image={{ .pauseContainerImage }}
sed -i  "s|$sandbox_image_line|sandbox_image = \"$pause_image\"|g" $FILE
