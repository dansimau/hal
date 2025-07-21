#!/usr/bin/env python3
"""
Colorizes Go test output from JSON format.

Reads JSON test output from stdin and outputs colored text:
- Regular output: light grey to stdout
- Lines with "FAIL": red to stderr  
- Failed test output: yellow to stderr (buffered and replayed)
"""

import sys
import json
from collections import defaultdict


class Colors:
    LIGHT_GREY = '\033[90m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    RESET = '\033[0m'


def main():
    # Buffer to store output for each test
    test_buffers = defaultdict(list)
    failed_tests = set()
    
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
            
        try:
            data = json.loads(line)
        except json.JSONDecodeError:
            continue
            
        package = data.get('Package', '')
        test = data.get('Test', '')
        action = data.get('Action', '')
        output = data.get('Output', '')
        
        # Create test key for buffering
        test_key = f"{package}::{test}" if test else package
        
        # Add output to buffer for this test (only if there's output)
        if test and output:
            test_buffers[test_key].append(output)
        
        # Check if this is a fail action
        if action == 'fail':
            failed_tests.add(test_key)
            # Output buffered content in yellow to stderr
            for buffered_output in test_buffers[test_key]:
                sys.stderr.write(f"{Colors.YELLOW}{buffered_output}{Colors.RESET}")
                sys.stderr.flush()
        
        # Only process output if it exists
        if output:
            # Check for lines containing "FAIL" - output in red to stderr
            if 'FAIL' in output:
                sys.stderr.write(f"{Colors.RED}{output}{Colors.RESET}")
                sys.stderr.flush()
            
            # Check if test is already failed - output in yellow to stderr
            elif test_key in failed_tests:
                sys.stderr.write(f"{Colors.YELLOW}{output}{Colors.RESET}")
                sys.stderr.flush()
            
            # Regular output - light grey to stdout
            else:
                sys.stdout.write(f"{Colors.LIGHT_GREY}{output}{Colors.RESET}")
                sys.stdout.flush()
        
        # Clean up buffer when test finishes (has Elapsed field)
        if 'Elapsed' in data:
            if test_key in test_buffers:
                del test_buffers[test_key]
            failed_tests.discard(test_key)


if __name__ == '__main__':
    main()