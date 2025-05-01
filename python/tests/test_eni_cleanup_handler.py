"""
Tests for the ENI cleanup handler.
"""

import unittest
import pulumi
from src.eni_cleanup_handler import register_eni_cleanup_handler
import pulumi_command as command

# Mocks for Pulumi testing
class PulumiMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + '_id', args.inputs]
    
    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}

# Apply the mocks
pulumi.runtime.set_mocks(PulumiMocks())

class TestENICleanupHandler(unittest.TestCase):
    def test_register_eni_cleanup_handler(self):
        """Test that register_eni_cleanup_handler creates a command resource."""
        
        def pulumi_program():
            # Create a dummy resource
            dummy_resource = pulumi.CustomResource("custom:resource:Dummy", "dummy")
            
            # Register the cleanup handler
            cleanup_command = register_eni_cleanup_handler(dummy_resource, ["us-east-1"])
            
            # Check that the command resource was created with the correct properties
            self.assertIsInstance(cleanup_command, command.local.Command)
            self.assertTrue(hasattr(cleanup_command, "create"))
            self.assertTrue(hasattr(cleanup_command, "delete"))
            
            # Return a value so pulumi.runtime.run_test can assert on it
            return cleanup_command.create
        
        # Run the Pulumi program
        result = pulumi.runtime.run_test(pulumi_program)
        self.assertEqual(result, "echo 'ENI cleanup handler attached'")

if __name__ == '__main__':
    unittest.main()