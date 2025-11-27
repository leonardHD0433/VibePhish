// labels is a map of campaign statuses to
// CSS classes
var labels = {
    "In progress": "label-primary",
    "Queued": "label-info",
    "Completed": "label-success",
    "Emails Sent": "label-success",
    "Error": "label-danger"
}

var campaigns = []
var campaign = {}

// Launch attempts to POST to /campaigns/
function launch() {
    // Validate fields BEFORE showing confirmation dialog
    var errorMessage = "";

    var name = $("#name").val();
    var template = $("#template").val();
    var page = $("#page").val();
    var profile = $("#profile").val();
    var users = $("#users").val();
    var launchDate = $("#launch_date").val();
    var sendByDate = $("#send_by_date").val();

    // console.log("Validation - name:", name);
    // console.log("Validation - template:", template);
    // console.log("Validation - page:", page);
    // console.log("Validation - profile:", profile);
    // console.log("Validation - users:", users);
    // console.log("Validation - launchDate:", launchDate);
    // console.log("Validation - sendByDate:", sendByDate);

    if (!name || name.trim() === "") {
        errorMessage = "Campaign name is required";
    } else if (!template || template === "") {
        errorMessage = "Email template is required";
    } else if (!page || page === "") {
        errorMessage = "Landing page is required";
    } else if (!profile || profile === "") {
        errorMessage = "Email type is required";
    } else if (!users || users.length === 0) {
        errorMessage = "At least one group is required";
    } else if (!launchDate || launchDate.trim() === "") {
        errorMessage = "Launch date is required";
    } else {
        // Validate launch_date is not in the past
        var launchMoment = moment(launchDate, "MMMM Do YYYY, h:mm a");
        var now = moment();

        if (launchMoment.isBefore(now)) {
            errorMessage = "Launch date cannot be in the past";
        } else if (sendByDate && sendByDate.trim() !== "") {
            // Validate send_by_date is after launch_date
            var sendByMoment = moment(sendByDate, "MMMM Do YYYY, h:mm a");

            // console.log("Launch date parsed:", launchMoment.format());
            // console.log("Send by date parsed:", sendByMoment.format());

            if (sendByMoment.isBefore(launchMoment)) {
                errorMessage = "The launch date must be before the \"send emails by\" date";
            }
        }
    }

    // If validation fails, show error and return
    if (errorMessage) {
        // console.log("Validation failed:", errorMessage);
        Swal.fire({
            title: "Validation Error",
            text: errorMessage,
            type: "error",
            confirmButtonColor: "#428bca"
        });
        return;
    }

    // console.log("Validation passed, checking rate limits");

    // Validate rate limit before showing confirmation
    validateRateLimit(name, template, page, profile, users, launchDate, sendByDate, function(rateLimitPassed, adjustedSendByDate) {
        if (!rateLimitPassed) {
            return; // User cancelled or validation failed
        }

        // Update sendByDate if it was adjusted
        if (adjustedSendByDate) {
            sendByDate = adjustedSendByDate;
            $("#send_by_date").val(adjustedSendByDate);
        }

        // All validation passed, show confirmation dialog
        Swal.fire({
            title: "Are you sure?",
            text: "This will schedule the campaign to be launched.",
            type: "question",
            animation: false,
            showCancelButton: true,
            confirmButtonText: "Launch",
            confirmButtonColor: "#428bca",
            reverseButtons: true
        }).then(function (result) {
            if (result.value) {
            // Flag to track if user dismissed the dialog
            var userDismissed = false;
            var timeoutId = null;

            // Show loading dialog
            Swal.fire({
                title: 'Launching Campaign...',
                html: '<div style="text-align: center;"><i class="fa fa-spinner fa-spin fa-3x"></i><br><br>Please wait while we schedule your campaign...</div>',
                allowOutsideClick: true,
                allowEscapeKey: true,
                allowEnterKey: false,
                showConfirmButton: false,
                showCancelButton: true,
                cancelButtonText: 'Cancel',
                cancelButtonColor: '#d33',
                onClose: function() {
                    // User closed the dialog - set flag and clear timeout
                    userDismissed = true;
                    if (timeoutId) {
                        clearTimeout(timeoutId);
                    }
                }
            });

            // Build campaign data
            groups = []
            $("#users").select2("data").forEach(function (group) {
                groups.push({
                    name: group.text
                });
            })

            var send_by_date = $("#send_by_date").val()
            if (send_by_date != "") {
                send_by_date = moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format()
            }

            campaign = {
                name: $("#name").val(),
                template: {
                    name: $("#template").select2("data")[0].text
                },
                url: $("#url").val(),
                page: {
                    name: $("#page").select2("data")[0].text
                },
                email_type: $("#profile").val(),
                launch_date: moment($("#launch_date").val(), "MMMM Do YYYY, h:mm a").utc().format(),
                send_by_date: send_by_date || null,
                groups: groups,
            }

            // Set up timeout (20 seconds)
            timeoutId = setTimeout(function() {
                if (!userDismissed) {
                    userDismissed = true;
                    Swal.fire({
                        title: "Request Timed Out",
                        text: "The campaign launch is taking longer than expected. Please check if the email service is running and try again.",
                        type: "error",
                        confirmButtonColor: "#428bca"
                    });
                }
            }, 20000);

            // Submit the campaign
            api.campaigns.post(campaign)
                .success(function (data) {
                    if (timeoutId) {
                        clearTimeout(timeoutId);
                    }
                    // Only show success if user hasn't dismissed
                    if (!userDismissed) {
                        campaign = data;
                        Swal.fire(
                            'Campaign Scheduled!',
                            'This campaign has been scheduled for launch!',
                            'success'
                        ).then(function() {
                            window.location = "/campaigns/" + campaign.id.toString()
                        });
                    }
                })
                .error(function (data) {
                    if (timeoutId) {
                        clearTimeout(timeoutId);
                    }

                    // Only show error if user hasn't dismissed
                    if (!userDismissed) {
                        // Enhanced error message extraction
                        var errorMessage = "An error occurred while launching the campaign";

                        // Try to extract error message from various response formats
                        if (data.responseJSON) {
                            if (data.responseJSON.message) {
                                errorMessage = data.responseJSON.message;
                            } else if (data.responseJSON.error) {
                                errorMessage = data.responseJSON.error;
                            } else if (typeof data.responseJSON === 'string') {
                                errorMessage = data.responseJSON;
                            }
                        } else if (data.responseText) {
                            try {
                                var errorData = JSON.parse(data.responseText);
                                errorMessage = errorData.message || errorData.error || errorMessage;
                            } catch (e) {
                                // If JSON parsing fails, use the raw text if it's not too long
                                if (data.responseText.length < 200) {
                                    errorMessage = data.responseText;
                                } else if (data.statusText) {
                                    errorMessage = data.statusText;
                                }
                            }
                        } else if (data.statusText) {
                            errorMessage = data.statusText;
                        }

                        // Add HTTP status code to error message for debugging
                        if (data.status) {
                            errorMessage = "[HTTP " + data.status + "] " + errorMessage;
                        }

                        Swal.fire({
                            title: "Launch Failed",
                            text: errorMessage,
                            type: "error",
                            confirmButtonColor: "#428bca"
                        });
                    }
                })
            }
        })
    });
}

// validateRateLimit checks if the campaign's send-by date meets rate limiting requirements
// Calls the API endpoint and shows a warning modal if the rate is too aggressive
function validateRateLimit(name, template, page, profile, users, launchDate, sendByDate, callback) {
    // Extract group IDs from users selection
    var groupIDs = [];
    $("#users").select2("data").forEach(function (group) {
        groupIDs.push(parseInt(group.id));
    });

    // Convert dates to ISO 8601 format for API
    var launchDateISO = moment(launchDate, "MMMM Do YYYY, h:mm a").utc().format();
    var sendByDateISO = sendByDate && sendByDate.trim() !== ""
        ? moment(sendByDate, "MMMM Do YYYY, h:mm a").utc().format()
        : "";

    // Call rate limit validation API
    $.ajax({
        url: "/api/campaigns/validate-rate-limit",
        method: "POST",
        data: JSON.stringify({
            launch_date: launchDateISO,
            send_by_date: sendByDateISO || null,
            group_ids: groupIDs
        }),
        contentType: "application/json",
        dataType: "json",
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
    .done(function(data) {
        if (data.success) {
            // Rate limit is acceptable - proceed
            callback(true, null);
        } else if (data.warning) {
            // Rate limit is too aggressive - show warning modal
            showRateLimitWarning(data.warning, function(accepted, useMinimum) {
                if (!accepted) {
                    // User cancelled
                    callback(false, null);
                } else if (useMinimum) {
                    // User accepted and wants to use minimum safe date
                    var adjustedDate = moment(data.warning.minimum_send_by_date).format("MMMM Do YYYY, h:mm a");
                    callback(true, adjustedDate);
                } else {
                    // User accepted but wants to keep their date (risky)
                    callback(true, null);
                }
            });
        } else {
            // Unknown response format
            callback(true, null);
        }
    })
    .fail(function(xhr, status, error) {
        // API call failed - log error but proceed with campaign creation
        console.error("Rate limit validation failed:", error);
        callback(true, null);
    });
}

// showRateLimitWarning displays a warning modal about aggressive send rates
function showRateLimitWarning(warning, callback) {
    var warningHTML = `
        <div style="text-align: left; margin-bottom: 20px;">
            <p><strong>⚠️ Your campaign will send emails too quickly!</strong></p>
            <p>${warning.warning_message}</p>
            <hr>
            <p><strong>Your Settings:</strong></p>
            <ul>
                <li>Sending to: <strong>${warning.total_recipients}</strong> recipients</li>
                <li>Interval: <strong>${warning.provided_interval_seconds.toFixed(1)}</strong> seconds per recipient</li>
                <li>Send-by date: <strong>${moment(warning.provided_send_by_date).format("MMMM Do YYYY, h:mm a")}</strong></li>
            </ul>
            <hr>
            <p><strong>Recommended Safe Settings:</strong></p>
            <ul>
                <li>Interval: <strong>${warning.minimum_interval_seconds}</strong> seconds per recipient (${(warning.minimum_interval_seconds / 60).toFixed(1)} minutes)</li>
                <li>Send-by date: <strong>${moment(warning.minimum_send_by_date).format("MMMM Do YYYY, h:mm a")}</strong></li>
                <li>Total duration: <strong>${warning.recommended_duration}</strong></li>
            </ul>
        </div>
    `;

    Swal.fire({
        title: "⚠️ Rate Limit Warning",
        html: warningHTML,
        type: "warning",
        showCancelButton: true,
        showDenyButton: true,
        confirmButtonText: "Use Safe Settings",
        denyButtonText: "Keep My Settings (Risky)",
        cancelButtonText: "Cancel",
        confirmButtonColor: "#28a745",
        denyButtonColor: "#ffc107",
        cancelButtonColor: "#6c757d",
        reverseButtons: true
    }).then(function (result) {
        if (result.value) {
            // User chose to use safe settings
            callback(true, true);
        } else if (result.isDenied) {
            // User chose to keep their risky settings
            callback(true, false);
        } else {
            // User cancelled
            callback(false, false);
        }
    });
}

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    try {
        console.log("sendTestEmail() called")

        // Clear any previous error messages
        $("#sendTestEmailModal\\.flashes").empty()

        // Check if email type is selected
        var emailType = $("#profile").val()
        console.log("Email type:", emailType)

        if (!emailType || emailType === "") {
            console.log("No email type selected")
            $("#sendTestEmailModal\\.flashes").append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
                <i class=\"fa fa-exclamation-circle\"></i> Please select an email type</div>")
            return
        }

        // Get template name - use empty string if not selected (backend will use default)
        var templateName = ""
        try {
            var templateData = $("#template").select2("data")
            if (templateData && templateData.length > 0) {
                templateName = templateData[0].text
            }
        } catch (e) {
            console.log("Could not get template from select2, using empty string (will use default)")
        }
        console.log("Template name:", templateName)

        // Get page name - use empty string if not selected
        var pageName = ""
        try {
            var pageData = $("#page").select2("data")
            if (pageData && pageData.length > 0) {
                pageName = pageData[0].text
            }
        } catch (e) {
            console.log("Could not get page from select2, using empty string")
        }
        console.log("Page name:", pageName)

        var test_email_request = {
            template: {
                name: templateName
            },
            first_name: $("input[name=to_first_name]").val(),
            last_name: $("input[name=to_last_name]").val(),
            email: $("input[name=to_email]").val(),
            position: $("input[name=to_position]").val(),
            url: $("#url").val(),
            page: {
                name: pageName
            },
            email_type: emailType
        }

        console.log("Test email request:", test_email_request)

        btnHtml = $("#sendTestModalSubmit").html()
        $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')

        console.log("Calling api.send_test_email()")

        // Send the test email
        api.send_test_email(test_email_request)
        .success(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">\
            <i class=\"fa fa-check-circle\"></i> Email Sent!</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
    } catch (error) {
        console.error("Error in sendTestEmail():", error)
        $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> Error: " + error.message + "</div>")
        if (typeof btnHtml !== 'undefined') {
            $("#sendTestModalSubmit").html(btnHtml)
        }
    }
}

function dismiss() {
    $("#modal\\.flashes").empty();
    $("#name").val("");
    $("#template").val("").change();
    $("#page").val("").change();
    $("#url").val("");
    $("#profile").val("").change();
    $("#users").val("").change();
    $("#modal").modal('hide');
}

function deleteCampaign(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + campaigns[idx].name,
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaigns[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Campaign Deleted!',
                'This campaign has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

// Set launch date to current time + 5 minutes (for manual/n8n sending mode)
function setLaunchDateToNowPlusFiveMinutes() {
    var launchDate = moment().add(5, 'minutes');
    $("#launch_date").data("DateTimePicker").date(launchDate);
    console.log("Auto-set launch date to:", launchDate.format("MMMM Do YYYY, h:mm a"));
}

function setupOptions() {
    api.groups.summary()
        .success(function (summaries) {
            groups = summaries.groups
            var group_s2 = $.map(groups, function (obj) {
                obj.text = obj.name
                obj.title = obj.num_targets + " targets"
                return obj
            });

            // Always initialize Select2 to prevent large multi-select box
            // Use dropdownParent: $('body') to prevent dropdown from being clipped by modal overflow
            $("#users.form-control").select2({
                placeholder: "Select Groups",
                data: group_s2,
                dropdownParent: $('body')
            });

            if (groups.length == 0) {
                modalError("No groups found!")
                return false;
            }
        });
    api.templates.get()
        .success(function (templates) {
            if (templates.length == 0) {
                modalError("No templates found!")
                return false
            } else {
                var template_s2 = $.map(templates, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var template_select = $("#template.form-control")
                template_select.select2({
                    placeholder: "Select a Template",
                    data: template_s2,
                });
                if (templates.length === 1) {
                    template_select.val(template_s2[0].id)
                    template_select.trigger('change.select2')
                }
            }
        });
    api.pages.get()
        .success(function (pages) {
            if (pages.length == 0) {
                modalError("No pages found!")
                return false
            } else {
                var page_s2 = $.map(pages, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var page_select = $("#page.form-control")
                page_select.select2({
                    placeholder: "Select a Landing Page",
                    data: page_s2,
                });
                if (pages.length === 1) {
                    page_select.val(page_s2[0].id)
                    page_select.trigger('change.select2')
                }
            }
        });
    api.email_types.get()
        .success(function (types) {
            if (types.length == 0) {
                modalError("No email types found!")
                return false
            } else {
                var profile_s2 = $.map(types, function (obj) {
                    obj.text = obj.display_name
                    obj.id = obj.value
                    return obj
                });
                var profile_select = $("#profile")
                profile_select.select2({
                    placeholder: "Select an Email Type",
                    data: profile_s2,
                    dropdownParent: $('#modal')
                });
                // Set default value to first email type
                profile_select.val(profile_s2[0].id).trigger('change');

                // Auto-update launch date when email type changes
                profile_select.on('change', function() {
                    setLaunchDateToNowPlusFiveMinutes();
                });

                // Auto-set launch date to now + 5 minutes when modal opens
                setLaunchDateToNowPlusFiveMinutes();
            }
        });
}

function edit(campaign) {
    setupOptions();
}

function copy(idx) {
    setupOptions();
    // Set our initial values
    api.campaignId.get(campaigns[idx].id)
        .success(function (campaign) {
            $("#name").val("Copy of " + campaign.name)
            if (!campaign.template.id) {
                $("#template").val("").change();
                $("#template").select2({
                    placeholder: campaign.template.name
                });
            } else {
                $("#template").val(campaign.template.id.toString());
                $("#template").trigger("change.select2")
            }
            if (!campaign.page.id) {
                $("#page").val("").change();
                $("#page").select2({
                    placeholder: campaign.page.name
                });
            } else {
                $("#page").val(campaign.page.id.toString());
                $("#page").trigger("change.select2")
            }
            if (campaign.email_type) {
                $("#profile").val(campaign.email_type);
                $("#profile").trigger("change.select2")
            }
            $("#url").val(campaign.url)
        })
        .error(function (data) {
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
        })
}

// ===================================================================
// AI-Assisted Campaign Creation Functions
// ===================================================================

var currentCampaignMode = 'copilot'; // Default mode
var chatHistory = [];

// Switch between campaign creation modes with smooth morphing animations
function switchCampaignMode(mode) {
    currentCampaignMode = mode;

    // Get the modal dialog element (this is what needs the width classes)
    var $modal = $('#modal');
    var $modalDialog = $modal.find('.modal-dialog');

    // Add morphing class for animation state
    $modalDialog.addClass('morphing');

    // Remove all mode classes and add the new one to the modal-dialog
    $modalDialog.removeClass('mode-manual mode-copilot mode-auto');
    $modalDialog.addClass('mode-' + mode);

    // Update toggle buttons with smooth transition
    $('.mode-toggle-btn').removeClass('active');
    $('[data-mode="' + mode + '"]').addClass('active');

    // Update info badge with animation
    if (mode === 'copilot') {
        $('.info-badge').removeClass('auto-mode').addClass('copilot-mode');
        $('.info-badge i').attr('class', 'fa fa-magic');
        $('#chat-mode-text').text('Copilot Mode - AI assists you in creating the campaign');
    } else if (mode === 'auto') {
        $('.info-badge').removeClass('copilot-mode').addClass('auto-mode');
        $('.info-badge i').attr('class', 'fa fa-rocket');
        $('#chat-mode-text').text('Auto Mode - AI creates the campaign automatically');
    }

    // Show/hide appropriate interfaces with smooth animations
    if (mode === 'manual') {
        $('#ai-chat-interface').fadeOut(300, function() {
            $('#manual-form-interface').fadeIn(300);
        });
        // Ensure form options are loaded when switching to manual mode
        // Check if select2 has been initialized
        if (!$('#template').hasClass('select2-hidden-accessible')) {
            setupOptions();
        }
    } else {
        $('#manual-form-interface').fadeOut(300, function() {
            $('#ai-chat-interface').fadeIn(300, function() {
                // Initialize n8n chat widget after interface is visible
                if (typeof window.initN8nChat === 'function') {
                    window.initN8nChat(mode);
                }
            });
        });
    }

    // Remove morphing class after animation completes (500ms)
    setTimeout(function() {
        $modalDialog.removeClass('morphing');
    }, 500);
}

// Reset chat interface to initial state
function resetChatInterface() {
    chatHistory = [];
    $('#chatMessages').html(`
        <div class="chat-message ai-message">
            <div class="message-avatar">
                <i class="fa fa-robot"></i>
            </div>
            <div class="message-content">
                <p><strong>FYPhish AI Assistant</strong></p>
                <p>Hello! I'm here to help you create an effective phishing campaign. Let's start by understanding your goals.</p>
                <p>What type of campaign would you like to create?</p>
                <div class="quick-suggestions">
                    <button class="suggestion-btn" onclick="sendQuickReply('Credential harvesting campaign')">
                        <i class="fa fa-key"></i> Credential Harvesting
                    </button>
                    <button class="suggestion-btn" onclick="sendQuickReply('Link clicking awareness')">
                        <i class="fa fa-link"></i> Link Awareness
                    </button>
                    <button class="suggestion-btn" onclick="sendQuickReply('Attachment awareness')">
                        <i class="fa fa-paperclip"></i> Attachment Awareness
                    </button>
                    <button class="suggestion-btn" onclick="sendQuickReply('Custom campaign')">
                        <i class="fa fa-cog"></i> Custom
                    </button>
                </div>
            </div>
        </div>
    `);
    $('#campaignPreview').hide();
}

// Send a chat message
function sendChatMessage() {
    var message = $('#chatInput').val().trim();
    if (!message) return;

    // Add user message to chat
    addChatMessage('user', message);

    // Clear input
    $('#chatInput').val('');

    // Show typing indicator
    showTypingIndicator();

    // Start autopilot agent workflow
    startAutopilotWorkflow(message);
}

// Send a quick reply (suggestion button click)
function sendQuickReply(message) {
    $('#chatInput').val(message);
    sendChatMessage();
}

// Add a message to the chat
function addChatMessage(sender, message) {
    var isUser = sender === 'user';
    var avatarIcon = isUser ? 'fa-user' : 'fa-robot';
    var messageClass = isUser ? 'user-message' : 'ai-message';

    var messageHTML = `
        <div class="chat-message ${messageClass}">
            <div class="message-avatar">
                <i class="fa ${avatarIcon}"></i>
            </div>
            <div class="message-content">
                <p>${escapeHtml(message)}</p>
            </div>
        </div>
    `;

    $('#chatMessages').append(messageHTML);
    scrollChatToBottom();

    // Store in history
    chatHistory.push({sender: sender, message: message});
}

// Show typing indicator
function showTypingIndicator() {
    var typingHTML = `
        <div class="chat-message ai-message typing-message">
            <div class="message-avatar">
                <i class="fa fa-robot"></i>
            </div>
            <div class="message-content">
                <div class="typing-indicator">
                    <div class="typing-dot"></div>
                    <div class="typing-dot"></div>
                    <div class="typing-dot"></div>
                </div>
            </div>
        </div>
    `;
    $('#chatMessages').append(typingHTML);
    scrollChatToBottom();
}

// Hide typing indicator
function hideTypingIndicator() {
    $('.typing-message').remove();
}

// Scroll chat to bottom
function scrollChatToBottom() {
    var chatMessages = $('#chatMessages');
    chatMessages.scrollTop(chatMessages[0].scrollHeight);
}

// ===================================================================
// Autopilot Agent Workflow Functions
// ===================================================================

// Global state for autopilot workflow
var autopilotState = {
    userPrompt: '',
    emailType: null,
    emailTypeName: '',
    groupId: null,
    groupName: '',
    targetCount: 0,
    templateId: null,
    templateName: '',
    pageId: null,
    pageName: '',
    aiGenerated: false
};

// Start the progressive autopilot workflow
function startAutopilotWorkflow(userPrompt) {
    // Reset state
    autopilotState = {
        userPrompt: userPrompt,
        emailType: null,
        emailTypeName: '',
        groupId: null,
        groupName: '',
        targetCount: 0,
        templateId: null,
        templateName: '',
        pageId: null,
        pageName: '',
        aiGenerated: false
    };

    // Step 1: Call AI Workflow 1 (Email Type Matching)
    callAutopilotAgent1(userPrompt);
}

// AI Workflow 1: Email Type Matching
function callAutopilotAgent1(userPrompt) {
    appendAIMessage('<i class="fa fa-cog fa-spin"></i> Analyzing email type from your request...');

    $.ajax({
        url: '/api/campaigns/ai-workflow/1',
        method: 'POST',
        data: JSON.stringify({
            user_prompt: userPrompt
        }),
        contentType: 'application/json',
        dataType: 'json',
        beforeSend: function(xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
    .done(function(data) {
        hideTypingIndicator();

        if (data.success) {
            autopilotState.emailType = data.matched_type;
            autopilotState.emailTypeName = data.email_type_name;

            var message = '✓ Email Type Identified: <strong>' + escapeHtml(data.email_type_name) + '</strong><br>';
            message += '<small class="text-muted">Confidence: ' + data.confidence + '%</small><br>';
            message += '<small class="text-muted">' + escapeHtml(data.reasoning) + '</small>';

            appendAIMessage(message);
            showTypingIndicator();

            // Step 2: Call AI Workflow 2 (Target Filtering)
            setTimeout(function() {
                callAutopilotAgent2(userPrompt);
            }, 500);
        } else {
            appendAIMessage('<span class="text-danger">✗ Failed to identify email type: ' + escapeHtml(data.error) + '</span>');
        }
    })
    .fail(function(xhr, status, error) {
        hideTypingIndicator();
        var errorMsg = xhr.responseJSON && xhr.responseJSON.error ? xhr.responseJSON.error : error;
        appendAIMessage('<span class="text-danger">✗ Error calling AI Workflow 1: ' + escapeHtml(errorMsg) + '</span>');
    });
}

// AI Workflow 2: Target Filtering & Group Creation
function callAutopilotAgent2(userPrompt) {
    appendAIMessage('<i class="fa fa-cog fa-spin"></i> Filtering targets and creating group...');

    $.ajax({
        url: '/api/campaigns/ai-workflow/2',
        method: 'POST',
        data: JSON.stringify({
            user_prompt: userPrompt
        }),
        contentType: 'application/json',
        dataType: 'json',
        beforeSend: function(xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
    .done(function(data) {
        hideTypingIndicator();

        if (data.success) {
            autopilotState.groupId = data.group_id;
            autopilotState.groupName = data.group_name;
            autopilotState.targetCount = data.target_count;

            var message = '✓ Target Group Created: <strong>' + escapeHtml(data.group_name) + '</strong><br>';
            message += '<small class="text-muted">Targets: ' + data.target_count + ' recipients</small><br>';
            message += '<small class="text-muted">' + escapeHtml(data.filter_description) + '</small>';

            appendAIMessage(message);
            showTypingIndicator();

            // Step 3: Call AI Workflow 3 (Template & Landing Page)
            setTimeout(function() {
                callAutopilotAgent3(userPrompt, autopilotState.emailType);
            }, 500);
        } else {
            appendAIMessage('<span class="text-danger">✗ Failed to create target group: ' + escapeHtml(data.error) + '</span>');
        }
    })
    .fail(function(xhr, status, error) {
        hideTypingIndicator();
        var errorMsg = xhr.responseJSON && xhr.responseJSON.error ? xhr.responseJSON.error : error;
        appendAIMessage('<span class="text-danger">✗ Error calling AI Workflow 2: ' + escapeHtml(errorMsg) + '</span>');
    });
}

// AI Workflow 3: Template & Landing Page Generation
function callAutopilotAgent3(userPrompt, emailType) {
    appendAIMessage('<i class="fa fa-cog fa-spin"></i> Preparing email template and landing page...');

    $.ajax({
        url: '/api/campaigns/ai-workflow/3',
        method: 'POST',
        data: JSON.stringify({
            user_prompt: userPrompt,
            theme_description: userPrompt,
            email_type: emailType
        }),
        contentType: 'application/json',
        dataType: 'json',
        beforeSend: function(xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
    .done(function(data) {
        hideTypingIndicator();

        if (data.success) {
            autopilotState.templateId = data.template_id;
            autopilotState.templateName = data.template_name;
            autopilotState.pageId = data.page_id;
            autopilotState.pageName = data.page_name;
            autopilotState.aiGenerated = data.ai_generated;

            var message = '✓ Template & Landing Page Ready<br>';
            message += '<small class="text-muted">Template: ' + escapeHtml(data.template_name) + '</small><br>';
            message += '<small class="text-muted">Landing Page: ' + escapeHtml(data.page_name) + '</small>';

            if (data.ai_generated && data.warning) {
                message += '<br><span class="text-warning"><i class="fa fa-exclamation-triangle"></i> ' + escapeHtml(data.warning) + '</span>';
            }

            appendAIMessage(message);

            // Step 4: Show campaign preview
            setTimeout(function() {
                showAutopilotCampaignPreview();
            }, 500);
        } else {
            appendAIMessage('<span class="text-danger">✗ Failed to generate template/page: ' + escapeHtml(data.error) + '</span>');
        }
    })
    .fail(function(xhr, status, error) {
        hideTypingIndicator();
        var errorMsg = xhr.responseJSON && xhr.responseJSON.error ? xhr.responseJSON.error : error;
        appendAIMessage('<span class="text-danger">✗ Error calling AI Workflow 3: ' + escapeHtml(errorMsg) + '</span>');
    });
}

// Helper function to append AI message (supports HTML content)
function appendAIMessage(htmlContent) {
    var messageHTML = `
        <div class="chat-message ai-message">
            <div class="message-avatar">
                <i class="fa fa-robot"></i>
            </div>
            <div class="message-content">
                <p>${htmlContent}</p>
            </div>
        </div>
    `;

    $('#chatMessages').append(messageHTML);
    scrollChatToBottom();
}

// Show final campaign preview with all gathered data
function showAutopilotCampaignPreview() {
    var campaignName = 'AI Campaign - ' + moment().format('YYYY-MM-DD HH:mm');
    var launchDate = moment().add(5, 'minutes').format('MMMM Do YYYY, h:mm a');

    // Populate manual form silently in background
    $('#name').val(campaignName);
    $('#template').val(autopilotState.templateId.toString()).trigger('change.select2');
    $('#page').val(autopilotState.pageId.toString()).trigger('change.select2');
    $('#profile').val(autopilotState.emailType).trigger('change.select2');
    $('#users').val([autopilotState.groupId.toString()]).trigger('change.select2');
    $('#launch_date').val(launchDate);

    // Show preview panel
    var previewHTML = `
        <div class="row">
            <div class="col-md-6">
                <p><strong>Campaign Name:</strong><br>${escapeHtml(campaignName)}</p>
                <p><strong>Email Type:</strong><br>${escapeHtml(autopilotState.emailTypeName)}</p>
                <p><strong>Email Template:</strong><br>${escapeHtml(autopilotState.templateName)}${autopilotState.aiGenerated ? ' <span class="label label-info">AI Generated</span>' : ''}</p>
            </div>
            <div class="col-md-6">
                <p><strong>Landing Page:</strong><br>${escapeHtml(autopilotState.pageName)}${autopilotState.aiGenerated ? ' <span class="label label-info">AI Generated</span>' : ''}</p>
                <p><strong>Target Group:</strong><br>${escapeHtml(autopilotState.groupName)} (${autopilotState.targetCount} recipients)</p>
                <p><strong>Launch Date:</strong><br>${escapeHtml(launchDate)}</p>
            </div>
        </div>
    `;

    $('#previewContent').html(previewHTML);
    $('#campaignPreview').slideDown();

    // Add final AI message
    appendAIMessage('🎯 <strong>Campaign Ready!</strong> Review the details above and click "Launch Campaign" when ready, or "Edit Details" to make changes.');
}

// ===================================================================
// End Autopilot Agent Workflow Functions
// ===================================================================

// Show campaign preview
function showCampaignPreview(campaignData) {
    var previewHTML = `
        <div class="row">
            <div class="col-md-6">
                <p><strong>Campaign Name:</strong><br>${escapeHtml(campaignData.name)}</p>
                <p><strong>Type:</strong><br>${escapeHtml(campaignData.type)}</p>
                <p><strong>Email Template:</strong><br>${escapeHtml(campaignData.template)}</p>
            </div>
            <div class="col-md-6">
                <p><strong>Landing Page:</strong><br>${escapeHtml(campaignData.landingPage)}</p>
                <p><strong>Target Groups:</strong><br>${escapeHtml(campaignData.targetGroups)}</p>
                <p><strong>Launch Date:</strong><br>${escapeHtml(campaignData.launchDate)}</p>
            </div>
        </div>
    `;

    $('#previewContent').html(previewHTML);
    $('#campaignPreview').slideDown();

    // Populate manual form in background
    switchToManualFormSilently(campaignData);
}

// Populate manual form without showing it
function switchToManualFormSilently(campaignData) {
    $('#name').val(campaignData.name);
    // Additional form population will happen here when integrated
}

// Edit campaign details
function editCampaignDetails() {
    switchCampaignMode('manual');
}

// Handle Enter key in chat input
$(document).on('keydown', '#chatInput', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendChatMessage();
    }
});

// ===================================================================
// End AI-Assisted Campaign Creation Functions
// ===================================================================

$(document).ready(function () {
    // Setup mode toggle buttons
    $('.mode-toggle-btn').on('click', function() {
        var mode = $(this).data('mode');
        switchCampaignMode(mode);
    });

    // Initialize modal dialog with copilot mode class (without animation)
    $('#modal .modal-dialog').addClass('mode-copilot');

    // Clear error messages when Send Test Email modal is closed
    $('#sendTestEmailModal').on('hidden.bs.modal', function () {
        $("#sendTestEmailModal\\.flashes").empty()
    });

    // Initialize in copilot mode
    switchCampaignMode('copilot');

    $("#launch_date").datetimepicker({
        "widgetPositioning": {
            "vertical": "bottom"
        },
        "showTodayButton": true,
        "defaultDate": moment(),
        "format": "MMMM Do YYYY, h:mm a"
    })
    $("#send_by_date").datetimepicker({
        "widgetPositioning": {
            "vertical": "bottom"
        },
        "showTodayButton": true,
        "useCurrent": false,
        "format": "MMMM Do YYYY, h:mm a"
    })
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        // Keep track of the number of open modals
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        // if the z-index of this modal has been set, ignore.
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        // Increment the number of open modals
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });

    // Initialize n8n chat when modal is shown (for copilot/auto modes)
    $('#modal').on('shown.bs.modal', function () {
        if (currentCampaignMode !== 'manual' && typeof window.initN8nChat === 'function') {
            // Small delay to ensure container is ready
            setTimeout(function() {
                window.initN8nChat(currentCampaignMode);
            }, 100);
        }
    });
    api.campaigns.summary()
        .success(function (data) {
            campaigns = data.campaigns
            $("#loading").hide()
            if (campaigns.length > 0) {
                $("#campaignTable").show()
                $("#campaignTableArchive").show()

                activeCampaignsTable = $("#campaignTable").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });
                archivedCampaignsTable = $("#campaignTableArchive").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });
                rows = {
                    'active': [],
                    'archived': []
                }
                $.each(campaigns, function (i, campaign) {
                    label = labels[campaign.status] || "label-default";

                    //section for tooltips on the status of a campaign to show some quick stats
                    var launchDate;
                    if (moment(campaign.launch_date).isAfter(moment())) {
                        launchDate = "Scheduled to start: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate + "<br><br>" + "Number of recipients: " + campaign.stats.total
                    } else {
                        launchDate = "Launch Date: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate + "<br><br>" + "Number of recipients: " + campaign.stats.total + "<br><br>" + "Emails opened: " + campaign.stats.opened + "<br><br>" + "Emails clicked: " + campaign.stats.clicked + "<br><br>" + "Submitted Credentials: " + campaign.stats.submitted_data + "<br><br>" + "Errors : " + campaign.stats.error + "<br><br>" + "Reported : " + campaign.stats.email_reported
                    }

                    var row = [
                        escapeHtml(campaign.name),
                        moment(campaign.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"right\" data-html=\"true\" title=\"" + quickStats + "\">" + campaign.status + "</span>",
                        "<div class='pull-right'><a class='btn btn-primary' href='/campaigns/" + campaign.id + "' data-toggle='tooltip' data-placement='left' title='View Results'>\
                    <i class='fa fa-bar-chart'></i>\
                    </a>\
            <span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Campaign' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' onclick='deleteCampaign(" + i + ")' data-toggle='tooltip' data-placement='left' title='Delete Campaign'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]
                    if (campaign.status == 'Completed') {
                        rows['archived'].push(row)
                    } else {
                        rows['active'].push(row)
                    }
                })
                activeCampaignsTable.rows.add(rows['active']).draw()
                archivedCampaignsTable.rows.add(rows['archived']).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching campaigns")
        })
    // Select2 Defaults
    $.fn.select2.defaults.set("width", "100%");
    $.fn.select2.defaults.set("dropdownParent", $("#modal_body"));
    $.fn.select2.defaults.set("theme", "bootstrap");
    $.fn.select2.defaults.set("sorter", function (data) {
        return data.sort(function (a, b) {
            if (a.text.toLowerCase() > b.text.toLowerCase()) {
                return 1;
            }
            if (a.text.toLowerCase() < b.text.toLowerCase()) {
                return -1;
            }
            return 0;
        });
    })
})
