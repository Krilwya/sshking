//go:build darwin

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>
#include <string.h>

int sshking_biometric_available(void) {
    @autoreleasepool {
        LAContext *context = [[LAContext alloc] init];
        NSError *error = nil;
        return [context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&error] ? 1 : 0;
    }
}

int sshking_biometric_authenticate(const char *reason, char **error_message) {
    @autoreleasepool {
        LAContext *context = [[LAContext alloc] init];
        NSError *availabilityError = nil;
        if (![context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&availabilityError]) {
            if (error_message != NULL) {
                *error_message = strdup([[availabilityError localizedDescription] UTF8String]);
            }
            return 0;
        }

        NSString *prompt = [NSString stringWithUTF8String:reason];
        dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
        __block BOOL verified = NO;
        __block NSError *authenticationError = nil;
        [context evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
                localizedReason:prompt
                          reply:^(BOOL success, NSError *error) {
            verified = success;
            authenticationError = error;
            dispatch_semaphore_signal(semaphore);
        }];
        dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);

        if (!verified && error_message != NULL) {
            NSString *message = authenticationError == nil
                ? @"Touch ID verification failed"
                : [authenticationError localizedDescription];
            *error_message = strdup([message UTF8String]);
        }
        return verified ? 1 : 0;
    }
}
