#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Define the target installation directory
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="swissarmycli"
SOURCE_BINARY_PATH="./bin/${BINARY_NAME}" # Assuming your Makefile builds to ./bin/

# --- Safety Checks ---
# 1. Check if the new binary exists
if [ ! -f "${SOURCE_BINARY_PATH}" ]; then
    echo "Error: New binary not found at ${SOURCE_BINARY_PATH}"
    echo "Please build the project first (e.g., 'make build')."
    exit 1
fi

# 2. Check if the installation directory exists
if [ ! -d "${INSTALL_DIR}" ]; then
    echo "Error: Installation directory ${INSTALL_DIR} does not exist."
    echo "Please create it or ensure it's the correct path."
    exit 1
fi

# --- Installation Steps ---
echo "Preparing to install ${BINARY_NAME} to ${INSTALL_DIR}..."

# 1. Remove existing binary from the installation directory (if it exists)
#    Using sudo because /usr/local/bin typically requires root privileges.
if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    echo "Removing existing ${BINARY_NAME} from ${INSTALL_DIR}..."
    sudo rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    if [ $? -ne 0 ]; then
        echo "Error: Failed to remove existing binary. Check permissions or if the file is in use."
        exit 1
    fi
    echo "Existing binary removed."
else
    echo "No existing ${BINARY_NAME} found in ${INSTALL_DIR}. Skipping removal."
fi

# 2. Move the new binary to the installation directory
#    Using sudo for the move operation as well.
echo "Moving new ${BINARY_NAME} to ${INSTALL_DIR}..."
sudo mv "${SOURCE_BINARY_PATH}" "${INSTALL_DIR}/${BINARY_NAME}"
if [ $? -ne 0 ]; then
    echo "Error: Failed to move new binary to ${INSTALL_DIR}. Check permissions."
    # Attempt to restore the source binary if the move failed partway (though mv is usually atomic)
    if [ ! -f "${SOURCE_BINARY_PATH}" ] && [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        # This case is unlikely with mv, but as a precaution
        echo "Attempting to move binary back to source..."
        sudo mv "${INSTALL_DIR}/${BINARY_NAME}" "${SOURCE_BINARY_PATH}"
    fi
    exit 1
fi

# 3. (Optional but Recommended) Ensure the new binary is executable
echo "Setting execute permissions for ${INSTALL_DIR}/${BINARY_NAME}..."
sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
if [ $? -ne 0 ]; then
    echo "Warning: Failed to set execute permissions. You may need to do this manually."
    # Not exiting on this warning, as the file is moved.
fi

echo ""
echo "${BINARY_NAME} has been successfully installed to ${INSTALL_DIR}/${BINARY_NAME}"
echo "You can now run it as: ${BINARY_NAME}"

# Verify (optional)
if command -v ${BINARY_NAME} &> /dev/null && [ "$(command -v ${BINARY_NAME})" == "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    echo "Verification: '${BINARY_NAME}' command points to the new installation."
    ${BINARY_NAME} --version # Or some other quick command to test it
else
    echo "Verification: Could not confirm '${BINARY_NAME}' command points to the new installation. You might need to open a new terminal session."
fi

exit 0