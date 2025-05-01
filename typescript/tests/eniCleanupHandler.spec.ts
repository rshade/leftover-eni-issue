import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as command from '@pulumi/command';
import { registerENICleanupHandler, ENICleanupComponent, attachENICleanupHandler } from '../src';

// Mock Pulumi runtime for testing
pulumi.runtime.setMocks({
    newResource: function(args: pulumi.runtime.MockResourceArgs): {id: string, state: any} {
        return {
            id: args.inputs.name + '_id',
            state: args.inputs,
        };
    },
    call: function(args: pulumi.runtime.MockCallArgs) {
        return args.inputs;
    },
});

describe('ENI Cleanup Handler', () => {
    test('registerENICleanupHandler creates a command resource', async () => {
        const program = async () => {
            // Create a dummy resource
            const vpc = new aws.ec2.Vpc('test-vpc', {
                cidrBlock: '10.0.0.0/16',
            });
            
            // Register the cleanup handler
            const cleanupCommand = registerENICleanupHandler(vpc, ['us-east-1']);
            
            // Check that the command resource was created with the correct properties
            expect(cleanupCommand).toBeInstanceOf(command.local.Command);
            expect(cleanupCommand.create).toBeDefined();
            expect(cleanupCommand.delete).toBeDefined();
        };
        
        await pulumi.runtime.runPulumiProgram(program);
    });
    
    test('ENICleanupComponent attaches handler to itself', async () => {
        const program = async () => {
            // Create the component
            const eniCleanup = new ENICleanupComponent('test', {
                regions: ['us-east-1', 'us-west-2'],
            });
            
            // We can't directly test the internals, but we can ensure it's created successfully
            expect(eniCleanup).toBeInstanceOf(ENICleanupComponent);
        };
        
        await pulumi.runtime.runPulumiProgram(program);
    });
    
    test('attachENICleanupHandler attaches handler to resource', async () => {
        const program = async () => {
            // Create a dummy resource
            const vpc = new aws.ec2.Vpc('test-vpc', {
                cidrBlock: '10.0.0.0/16',
            });
            
            // We'll use a spy to verify attachENICleanupHandler calls registerENICleanupHandler
            const registerSpy = jest.spyOn(pulumi, 'CustomResource').mockImplementation((type, name, props, opts) => {
                return {} as pulumi.CustomResource;
            });
            
            // Attach the cleanup handler
            attachENICleanupHandler(vpc, { regions: ['us-east-1'] });
            
            // Verify registerENICleanupHandler was called
            expect(registerSpy).toHaveBeenCalled();
            
            // Clean up
            registerSpy.mockRestore();
        };
        
        await pulumi.runtime.runPulumiProgram(program);
    });
});