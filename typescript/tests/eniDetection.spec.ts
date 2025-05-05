import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import { detectOrphanedENIs, isLikelyOrphaned, logOrphanedENIsOnDestroy } from '../src/eniDetection';

// Mock Pulumi to avoid actual resource creation during tests
pulumi.runtime.setMocks({
    newResource: function(args: pulumi.runtime.MockResourceArgs): {id: string, state: any} {
        return {
            id: args.inputs.name + '_id',
            state: args.inputs,
        };
    },
    call: function(args: pulumi.runtime.MockCallArgs) {
        // Mock EC2 API calls
        if (args.token === 'aws:ec2/getNetworkInterfaces:getNetworkInterfaces') {
            // Return mock network interfaces
            return {
                networkInterfaces: [
                    {
                        id: 'eni-1234567890',
                        vpcId: 'vpc-1234567890',
                        subnetId: 'subnet-1234567890',
                        availabilityZone: 'us-east-1a',
                        description: 'Test ENI 1',
                        status: 'available',
                        tags: { Name: 'test-eni-1' },
                    },
                    {
                        id: 'eni-0987654321',
                        vpcId: 'vpc-1234567890',
                        subnetId: 'subnet-1234567890',
                        availabilityZone: 'us-east-1b',
                        description: 'Amazon EKS Test ENI',
                        status: 'available',
                        tags: { Name: 'test-eni-2' },
                    },
                    {
                        id: 'eni-1122334455',
                        vpcId: 'vpc-1234567890',
                        subnetId: 'subnet-1234567890',
                        availabilityZone: 'us-east-1c',
                        description: 'Test ENI 3',
                        status: 'in-use',
                        attachment: {
                            attachmentId: 'attachment-1234567890',
                            instanceId: 'i-1234567890',
                            status: 'attached',
                        },
                        tags: { Name: 'test-eni-3' },
                    },
                    {
                        id: 'eni-5566778899',
                        vpcId: 'vpc-1234567890',
                        subnetId: 'subnet-1234567890',
                        availabilityZone: 'us-east-1a',
                        description: 'Test ENI 4',
                        status: 'available',
                        tags: { 'kubernetes.io/cluster/test-cluster': 'owned', Name: 'test-eni-4' },
                    },
                ]
            };
        }
        return args.inputs;
    },
});

describe('ENI Detection Module', () => {
    test('isLikelyOrphaned should identify orphaned ENIs', () => {
        // Test with an available ENI that should be considered orphaned
        const orphanedENI1 = {
            id: 'eni-1234567890',
            status: 'available',
            description: 'Test ENI',
            tags: {},
        } as unknown as aws.ec2.NetworkInterface;
        expect(isLikelyOrphaned(orphanedENI1)).toBe(true);
        
        // Test with an ENI that has a reserved description and should not be considered orphaned
        const nonOrphanedENI1 = {
            id: 'eni-0987654321',
            status: 'available',
            description: 'Amazon EKS ENI',
            tags: {},
        } as unknown as aws.ec2.NetworkInterface;
        expect(isLikelyOrphaned(nonOrphanedENI1)).toBe(false);
        
        // Test with an ENI that is in use and should not be considered orphaned
        const nonOrphanedENI2 = {
            id: 'eni-1122334455',
            status: 'in-use',
            description: 'Test ENI',
            tags: {},
        } as unknown as aws.ec2.NetworkInterface;
        expect(isLikelyOrphaned(nonOrphanedENI2)).toBe(false);
        
        // Test with an ENI that has Kubernetes tags
        const orphanedENI2 = {
            id: 'eni-5566778899',
            status: 'available',
            description: 'Test ENI',
            tags: { 'kubernetes.io/cluster/test-cluster': 'owned' },
        } as unknown as aws.ec2.NetworkInterface;
        expect(isLikelyOrphaned(orphanedENI2)).toBe(true);
    });
    
    test('detectOrphanedENIs should return a list of orphaned ENIs', async () => {
        const results = await detectOrphanedENIs(['us-east-1']);
        
        // We expect to get two orphaned ENIs based on our mock data:
        // - eni-1234567890 (available with no reserved description)
        // - eni-5566778899 (has kubernetes tag)
        expect(results.length).toBe(2);
        
        // Verify the first orphaned ENI
        const eni1 = results.find(eni => eni.id === 'eni-1234567890');
        expect(eni1).toBeDefined();
        expect(eni1?.region).toBe('us-east-1');
        expect(eni1?.vpcId).toBe('vpc-1234567890');
        
        // Verify the second orphaned ENI
        const eni2 = results.find(eni => eni.id === 'eni-5566778899');
        expect(eni2).toBeDefined();
        expect(eni2?.region).toBe('us-east-1');
        expect(eni2?.vpcId).toBe('vpc-1234567890');
        
        // Verify that the EKS ENI was not included
        const eksENI = results.find(eni => eni.id === 'eni-0987654321');
        expect(eksENI).toBeUndefined();
        
        // Verify that the in-use ENI was not included
        const inUseENI = results.find(eni => eni.id === 'eni-1122334455');
        expect(inUseENI).toBeUndefined();
    });
    
    test('logOrphanedENIsOnDestroy should create a command resource', () => {
        const program = async () => {
            // Create a logger
            const logger = logOrphanedENIsOnDestroy('test-resource', undefined, ['us-east-1']);
            
            // Verify it's a command resource
            expect(logger.constructor.name).toBe('Command');
        };
        
        return pulumi.runtime.runPulumiProgram(program);
    });
});