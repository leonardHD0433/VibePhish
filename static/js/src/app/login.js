/**
 * FYPhish SSO-First Login Enhancement
 *
 * Provides advanced authentication functionality including:
 * - Emergency access via Konami code
 * - SSO error handling and retry logic
 * - Enhanced accessibility features
 * - Mobile-responsive interactions
 * - Security monitoring and logging
 */

(function() {
    'use strict';

    // Configuration constants
    const CONFIG = {
        KONAMI_CODE: [38, 38, 40, 40, 37, 39, 37, 39, 66, 65], // Up Up Down Down Left Right Left Right B A
        KONAMI_TIMEOUT: 3000, // 3 seconds to complete sequence
        SSO_RETRY_DELAY: 2000, // 2 seconds before showing retry options
        EMERGENCY_SESSION_KEY: 'emergency_access_granted',
        DEBUG_MODE: false // Enable for development debugging
    };

    // State management
    let konamiSequence = [];
    let konamiTimer = null;
    let emergencyAccessEnabled = false;
    let ssoAttemptCount = 0;
    let lastSSOAttempt = 0;

    /**
     * Enhanced Emergency Access Manager
     */
    const EmergencyAccess = {
        init: function() {
            this.bindEvents();
            this.checkExistingAccess();
        },

        bindEvents: function() {
            // Konami code listener
            document.addEventListener('keydown', this.handleKonamiInput.bind(this));

            // Alternative activation methods
            document.addEventListener('keydown', this.handleKeyboardShortcuts.bind(this));

            // Mouse/touch sequence (for mobile)
            this.initMobileActivation();

            // Admin development backdoor (only in debug mode)
            if (CONFIG.DEBUG_MODE && window.location.hostname === 'localhost') {
                this.enableDebugAccess();
            }
        },

        handleKonamiInput: function(event) {
            const keyCode = event.which || event.keyCode;

            konamiSequence.push(keyCode);

            // Clear timer and set new one
            if (konamiTimer) {
                clearTimeout(konamiTimer);
            }

            konamiTimer = setTimeout(() => {
                konamiSequence = [];
            }, CONFIG.KONAMI_TIMEOUT);

            // Check if sequence matches
            if (konamiSequence.length === CONFIG.KONAMI_CODE.length) {
                if (this.validateKonamiSequence()) {
                    this.activateEmergencyAccess('konami');
                    event.preventDefault();
                }
                konamiSequence = [];
            }
        },

        validateKonamiSequence: function() {
            return konamiSequence.every((key, index) => key === CONFIG.KONAMI_CODE[index]);
        },

        handleKeyboardShortcuts: function(event) {
            // Ctrl + Shift + E + M for Emergency
            if (event.ctrlKey && event.shiftKey && event.key === 'E') {
                const nextKey = () => {
                    const handler = (e) => {
                        if (e.key === 'M') {
                            this.activateEmergencyAccess('keyboard');
                            e.preventDefault();
                        }
                        document.removeEventListener('keydown', handler);
                    };
                    document.addEventListener('keydown', handler);
                    setTimeout(() => document.removeEventListener('keydown', handler), 2000);
                };
                nextKey();
            }
        },

        initMobileActivation: function() {
            let tapCount = 0;
            let tapTimer = null;
            const emergencyHint = document.getElementById('emergency-hint');

            if (emergencyHint) {
                emergencyHint.addEventListener('click', () => {
                    tapCount++;

                    if (tapTimer) {
                        clearTimeout(tapTimer);
                    }

                    tapTimer = setTimeout(() => {
                        tapCount = 0;
                    }, 1000);

                    // Seven taps to activate
                    if (tapCount === 7) {
                        this.activateEmergencyAccess('mobile');
                        tapCount = 0;
                    }
                });
            }
        },

        activateEmergencyAccess: function(method) {
            if (emergencyAccessEnabled) return;

            emergencyAccessEnabled = true;
            sessionStorage.setItem(CONFIG.EMERGENCY_SESSION_KEY, 'true');

            // Log activation for security monitoring
            this.logEmergencyAccess(method);

            // Show emergency access UI
            this.showEmergencyInterface();

            // Provide user feedback
            this.showActivationFeedback(method);
        },

        logEmergencyAccess: function(method) {
            const logData = {
                timestamp: new Date().toISOString(),
                method: method,
                userAgent: navigator.userAgent,
                ip: 'client-side', // Server will capture real IP
                url: window.location.href
            };

            // Send to server for security logging
            if (window.fetch) {
                fetch('/api/security/emergency-access-log', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Requested-With': 'XMLHttpRequest'
                    },
                    body: JSON.stringify(logData)
                }).catch(() => {
                    // Fallback to console for development
                    console.warn('Emergency access activated:', logData);
                });
            }
        },

        showEmergencyInterface: function() {
            const localLoginSection = document.getElementById('local-login-section');
            const emergencyToggle = document.getElementById('emergency-toggle');
            const usernameField = document.getElementById('username');

            if (localLoginSection) {
                localLoginSection.style.display = 'block';
                localLoginSection.setAttribute('aria-hidden', 'false');

                // Focus management for accessibility
                setTimeout(() => {
                    if (usernameField) {
                        usernameField.focus();
                        usernameField.setAttribute('required', 'true');
                    }
                }, 300);
            }

            if (emergencyToggle) {
                emergencyToggle.style.display = 'none';
            }
        },

        showActivationFeedback: function(method) {
            const feedbackMessages = {
                konami: 'Konami Code activated! Emergency access enabled.',
                keyboard: 'Keyboard shortcut activated! Emergency access enabled.',
                mobile: 'Touch sequence activated! Emergency access enabled.',
                debug: 'Debug mode: Emergency access enabled.'
            };

            // Create temporary feedback element
            const feedback = document.createElement('div');
            feedback.className = 'emergency-activation-feedback';
            feedback.innerHTML = `
                <div class="alert alert-info" role="alert">
                    <i class="fa fa-info-circle" aria-hidden="true"></i>
                    ${feedbackMessages[method] || 'Emergency access enabled.'}
                </div>
            `;

            // Insert before form
            const form = document.querySelector('.form-signin');
            if (form && form.parentNode) {
                form.parentNode.insertBefore(feedback, form);

                // Auto-remove after 5 seconds
                setTimeout(() => {
                    if (feedback && feedback.parentNode) {
                        feedback.parentNode.removeChild(feedback);
                    }
                }, 5000);
            }
        },

        checkExistingAccess: function() {
            const hasAccess = sessionStorage.getItem(CONFIG.EMERGENCY_SESSION_KEY);
            const urlHasEmergency = window.location.search.includes('emergency=true');

            if (hasAccess || urlHasEmergency) {
                this.activateEmergencyAccess('session');
            }
        },

        enableDebugAccess: function() {
            // Debug mode: Triple-click logo for immediate access
            const logo = document.getElementById('logo');
            if (logo) {
                let clickCount = 0;
                logo.addEventListener('click', () => {
                    clickCount++;
                    setTimeout(() => { clickCount = 0; }, 1000);

                    if (clickCount === 3) {
                        this.activateEmergencyAccess('debug');
                    }
                });
            }
        }
    };

    /**
     * Enhanced SSO Manager
     */
    const SSOManager = {
        init: function() {
            this.bindEvents();
            this.checkSSOStatus();
            this.handleReturnFromSSO();
        },

        bindEvents: function() {
            const ssoButton = document.querySelector('.primary-sso-btn');

            if (ssoButton) {
                ssoButton.addEventListener('click', this.handleSSOClick.bind(this));
            }
        },

        handleSSOClick: function(event) {
            const button = event.currentTarget;
            const statusIndicator = document.getElementById('status-indicator');

            // Prevent double-clicks
            if (button.disabled) {
                event.preventDefault();
                return false;
            }

            // Rate limiting
            const now = Date.now();
            if (now - lastSSOAttempt < 1000) {
                event.preventDefault();
                return false;
            }

            lastSSOAttempt = now;
            ssoAttemptCount++;

            // Show loading state
            button.disabled = true;
            button.classList.add('loading');

            if (statusIndicator) {
                statusIndicator.style.display = 'flex';
            }

            // Store attempt for error detection
            sessionStorage.setItem('sso_attempt', JSON.stringify({
                timestamp: now,
                attempt: ssoAttemptCount,
                url: window.location.href
            }));

            // Re-enable button after timeout
            setTimeout(() => {
                button.disabled = false;
                button.classList.remove('loading');
            }, 5000);
        },

        handleReturnFromSSO: function() {
            const urlParams = new URLSearchParams(window.location.search);
            const ssoAttempt = sessionStorage.getItem('sso_attempt');

            if (ssoAttempt && (urlParams.get('error') || urlParams.get('sso_error'))) {
                const attempt = JSON.parse(ssoAttempt);
                const errorType = urlParams.get('error') || 'unknown';

                this.handleSSOError(errorType, attempt);
                sessionStorage.removeItem('sso_attempt');
            }
        },

        handleSSOError: function(errorType, attemptData) {
            const errorMessages = {
                'access_denied': 'Access denied. Please ensure you have the correct permissions.',
                'invalid_request': 'Invalid request. Please try again.',
                'server_error': 'Server error occurred. Please try again later.',
                'timeout': 'Authentication timed out. Please try again.',
                'unknown': 'Authentication failed. Please try again.'
            };

            const message = errorMessages[errorType] || errorMessages.unknown;
            this.showSSOError(message);

            // Auto-suggest emergency access after multiple failures
            if (attemptData.attempt >= 3) {
                this.suggestEmergencyAccess();
            }
        },

        showSSOError: function(message) {
            const ssoError = document.getElementById('sso-error');
            const errorMessage = document.getElementById('error-message');

            if (ssoError && errorMessage) {
                errorMessage.textContent = message;
                ssoError.style.display = 'flex';
                ssoError.setAttribute('role', 'alert');

                // Hide loading indicator
                const statusIndicator = document.getElementById('status-indicator');
                if (statusIndicator) {
                    statusIndicator.style.display = 'none';
                }

                // Auto-hide after 10 seconds
                setTimeout(() => {
                    ssoError.style.display = 'none';
                }, 10000);
            }
        },

        suggestEmergencyAccess: function() {
            const emergencyToggle = document.getElementById('emergency-toggle');

            if (emergencyToggle && !emergencyAccessEnabled) {
                emergencyToggle.innerHTML = `
                    <i class="fa fa-key" aria-hidden="true"></i>
                    <span>SSO having issues? Try emergency access</span>
                `;
                emergencyToggle.classList.add('emergency-fallback');
                emergencyToggle.style.display = 'block';

                // Make it more prominent
                emergencyToggle.style.animation = 'pulse 2s infinite';
            }
        },

        checkSSOStatus: function() {
            // Check if SSO service is available
            if (window.fetch) {
                fetch('/auth/microsoft/status', {
                    method: 'HEAD',
                    cache: 'no-cache'
                }).catch(() => {
                    // SSO service unavailable, suggest emergency access
                    this.showSSOError('SSO service is currently unavailable.');
                    this.suggestEmergencyAccess();
                });
            }
        }
    };

    /**
     * Accessibility Enhancements
     */
    const AccessibilityManager = {
        init: function() {
            this.setupKeyboardNavigation();
            this.setupScreenReaderSupport();
            this.setupFocusManagement();
        },

        setupKeyboardNavigation: function() {
            // Tab order management
            const focusableElements = document.querySelectorAll(
                'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'
            );

            // Ensure logical tab order
            focusableElements.forEach((element, index) => {
                if (!element.hasAttribute('tabindex')) {
                    element.setAttribute('tabindex', '0');
                }
            });
        },

        setupScreenReaderSupport: function() {
            // Live region for status updates
            const liveRegion = document.createElement('div');
            liveRegion.id = 'live-region';
            liveRegion.setAttribute('aria-live', 'polite');
            liveRegion.setAttribute('aria-atomic', 'true');
            liveRegion.className = 'sr-only';
            document.body.appendChild(liveRegion);

            // Function to announce messages
            window.announceToScreenReader = function(message) {
                liveRegion.textContent = message;
                setTimeout(() => {
                    liveRegion.textContent = '';
                }, 1000);
            };
        },

        setupFocusManagement: function() {
            // Trap focus within form when emergency access is active
            const form = document.querySelector('.form-signin');
            const localSection = document.getElementById('local-login-section');

            if (form && localSection) {
                form.addEventListener('keydown', (e) => {
                    if (e.key === 'Tab' && localSection.style.display !== 'none') {
                        const focusableInSection = localSection.querySelectorAll(
                            'input:not([disabled]), button:not([disabled])'
                        );

                        if (focusableInSection.length > 0) {
                            const firstFocusable = focusableInSection[0];
                            const lastFocusable = focusableInSection[focusableInSection.length - 1];

                            if (e.shiftKey) {
                                if (document.activeElement === firstFocusable) {
                                    lastFocusable.focus();
                                    e.preventDefault();
                                }
                            } else {
                                if (document.activeElement === lastFocusable) {
                                    firstFocusable.focus();
                                    e.preventDefault();
                                }
                            }
                        }
                    }
                });
            }
        }
    };

    /**
     * Form Enhancement Manager
     */
    const FormManager = {
        init: function() {
            this.setupFormValidation();
            this.setupProgressiveEnhancement();
            this.setupSecurityFeatures();
        },

        setupFormValidation: function() {
            const form = document.querySelector('.form-signin');

            if (form) {
                form.addEventListener('submit', this.handleFormSubmit.bind(this));
            }
        },

        handleFormSubmit: function(event) {
            const form = event.target;
            const emergencyLogin = form.querySelector('input[name="emergency_login"]');

            // Log emergency login attempts
            if (emergencyLogin && emergencyLogin.value === 'true') {
                this.logEmergencyLogin();
            }

            // Add submission timestamp
            const timestamp = document.createElement('input');
            timestamp.type = 'hidden';
            timestamp.name = 'submission_timestamp';
            timestamp.value = new Date().toISOString();
            form.appendChild(timestamp);
        },

        logEmergencyLogin: function() {
            if (window.announceToScreenReader) {
                window.announceToScreenReader('Emergency login attempt initiated');
            }

            // Add visual indicator
            const submitButton = document.querySelector('.emergency-login-btn');
            if (submitButton) {
                submitButton.innerHTML = '<i class="fa fa-spinner fa-spin" aria-hidden="true"></i> Authenticating...';
                submitButton.disabled = true;
            }
        },

        setupProgressiveEnhancement: function() {
            // Enhance with JavaScript, but ensure it works without
            const noJsElements = document.querySelectorAll('.no-js-only');
            noJsElements.forEach(element => {
                element.style.display = 'none';
            });

            // Show JS-enhanced elements
            const jsElements = document.querySelectorAll('.js-enhanced');
            jsElements.forEach(element => {
                element.style.display = 'block';
            });
        },

        setupSecurityFeatures: function() {
            // Prevent form data from being cached
            const form = document.querySelector('.form-signin');
            if (form) {
                form.setAttribute('autocomplete', 'off');
            }

            // Clear sensitive data on page unload
            window.addEventListener('beforeunload', () => {
                const passwordFields = document.querySelectorAll('input[type="password"]');
                passwordFields.forEach(field => {
                    field.value = '';
                });
            });
        }
    };

    /**
     * Initialize all managers when DOM is ready
     */
    function initializeLogin() {
        try {
            EmergencyAccess.init();
            SSOManager.init();
            AccessibilityManager.init();
            FormManager.init();

            // Development helpers
            if (CONFIG.DEBUG_MODE) {
                window.FYPhishLogin = {
                    EmergencyAccess,
                    SSOManager,
                    activateEmergency: () => EmergencyAccess.activateEmergencyAccess('manual')
                };
                console.log('FYPhish Login Debug Mode: Use FYPhishLogin.activateEmergency() for manual activation');
            }

        } catch (error) {
            console.error('Login initialization error:', error);
            // Ensure basic functionality still works
            document.body.classList.add('js-error');
        }
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initializeLogin);
    } else {
        initializeLogin();
    }

})();