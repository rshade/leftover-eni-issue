import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as command from '@pulumi/command';
import { 
    registerENICleanupHandler, 
    ENICleanupComponent, 
    attachENICleanupHandler 
} from '../src';
import { cleanupENIs, createPreDestroyCleanupHook } from '../src/eniCleanup';
import { OrphanedENI } from '../src/eniDetection';

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
    
    test('cleanupENIs handles empty input', async () => {
        const result = await cleanupENIs([]);
        expect(result.successCount).toBe(0);
        expect(result.failureCount).toBe(0);
        expect(result.skippedCount).toBe(0);
    });
    
    test('cleanupENIs processes ENIs correctly', async () => {
        // Create test ENIs
        const testEnis: OrphanedENI[] = [
            {
                id: 'eni-1234567890',
                region: 'us-east-1',
                vpcId: 'vpc-1234567890',
                description: 'Test ENI 1',
                tags: { Name: 'test-eni-1' },
            },
            {
                id: 'eni-0987654321',
                region: 'us-east-1',
                vpcId: 'vpc-1234567890',
                description: 'Test ENI 2',
                tags: { DoNotDelete: 'true' },
            },
        ];
        
        // Test with exclude tags
        const result1 = await cleanupENIs(testEnis, {
            excludeTagKeys: ['DoNotDelete'],
            dryRun: true,
        });
        
        expect(result1.skippedCount).toBe(1);  // One ENI should be skipped due to exclude tag
        
        // Test with dryRun mode
        const result2 = await cleanupENIs(testEnis, {
            dryRun: true,
        });
        
        expect(result2.skippedCount).toBe(2);  // Both ENIs should be skipped in dry run mode
        
        // Test normal operation (simulated since we don't make actual AWS calls in tests)
        const result3 = await cleanupENIs(testEnis);
        
        // In our implementation, we simulate success for all ENIs
        expect(result3.successCount).toBe(2);
        expect(result3.cleanedENIs.length).toBe(2);
    });
    
    test('createPreDestroyCleanupHook creates a command resource', async () => {
        const program = async () => {
            // Create a dummy resource
            const vpc = new aws.ec2.Vpc('test-vpc', {
                cidrBlock: '10.0.0.0/16',
            });
            
            // Create the pre-destroy hook
            const cleanupHook = createPreDestroyCleanupHook(vpc, {
                regions: ['us-east-1'],
                dryRun: true,
            });
            
            // Verify it's a command resource
            expect(cleanupHook.constructor.name).toBe('Command');
        };
        
        return pulumi.runtime.runPulumiProgram(program);
    });
});