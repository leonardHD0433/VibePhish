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
    Swal.fire({
        title: "Are you sure?",
        text: "This will schedule the campaign to be launched.",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Launch",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                groups = []
                $("#users").select2("data").forEach(function (group) {
                    groups.push({
                        name: group.text
                    });
                })
                // Validate our fields
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
                    smtp: {
                        name: $("#profile").select2("data")[0].text
                    },
                    launch_date: moment($("#launch_date").val(), "MMMM Do YYYY, h:mm a").utc().format(),
                    send_by_date: send_by_date || null,
                    groups: groups,
                }
                // Submit the campaign
                api.campaigns.post(campaign)
                    .success(function (data) {
                        resolve()
                        campaign = data
                    })
                    .error(function (data) {
                        $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
                        Swal.close()
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Campaign Scheduled!',
                'This campaign has been scheduled for launch!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            window.location = "/campaigns/" + campaign.id.toString()
        })
    })
}

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    var test_email_request = {
        template: {
            name: $("#template").select2("data")[0].text
        },
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        url: $("#url").val(),
        page: {
            name: $("#page").select2("data")[0].text
        },
        smtp: {
            name: $("#profile").select2("data")[0].text
        }
    }
    btnHtml = $("#sendTestModalSubmit").html()
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
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

function setupOptions() {
    api.groups.summary()
        .success(function (summaries) {
            groups = summaries.groups
            if (groups.length == 0) {
                modalError("No groups found!")
                return false;
            } else {
                var group_s2 = $.map(groups, function (obj) {
                    obj.text = obj.name
                    obj.title = obj.num_targets + " targets"
                    return obj
                });
                console.log(group_s2)
                $("#users.form-control").select2({
                    placeholder: "Select Groups",
                    data: group_s2,
                });
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
    api.SMTP.get()
        .success(function (profiles) {
            if (profiles.length == 0) {
                modalError("No profiles found!")
                return false
            } else {
                var profile_s2 = $.map(profiles, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var profile_select = $("#profile.form-control")
                profile_select.select2({
                    placeholder: "Select a Sending Profile",
                    data: profile_s2,
                }).select2("val", profile_s2[0]);
                if (profiles.length === 1) {
                    profile_select.val(profile_s2[0].id)
                    profile_select.trigger('change.select2')
                }
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
            if (!campaign.smtp.id) {
                $("#profile").val("").change();
                $("#profile").select2({
                    placeholder: campaign.smtp.name
                });
            } else {
                $("#profile").val(campaign.smtp.id.toString());
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
            $('#ai-chat-interface').fadeIn(300);
        });
        resetChatInterface();
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

    // Simulate AI response (TODO: Replace with actual LLM API call)
    setTimeout(function() {
        hideTypingIndicator();
        processAIResponse(message);
    }, 1500);
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

// Process AI response (Placeholder - will integrate with LLM)
function processAIResponse(userMessage) {
    var response = "I understand you want to create a campaign. Let me help you with that. ";

    if (userMessage.toLowerCase().includes('credential')) {
        response += "For credential harvesting, I recommend:\n\n";
        response += "1. A realistic login page template\n";
        response += "2. Targeting employees with access to sensitive systems\n";
        response += "3. Using a scenario like password expiration\n\n";
        response += "Would you like me to create this campaign for you?";

        var aiMessageHTML = `
            <div class="chat-message ai-message">
                <div class="message-avatar">
                    <i class="fa fa-robot"></i>
                </div>
                <div class="message-content">
                    <p>${response}</p>
                    <div class="quick-suggestions">
                        <button class="suggestion-btn" onclick="generateCampaign('credential_harvesting')">
                            <i class="fa fa-check"></i> Yes, create it
                        </button>
                        <button class="suggestion-btn" onclick="sendQuickReply('I need different options')">
                            <i class="fa fa-times"></i> Show alternatives
                        </button>
                    </div>
                </div>
            </div>
        `;
        $('#chatMessages').append(aiMessageHTML);
    } else {
        response += "Could you provide more details about:\n• Target audience\n• Campaign objectives\n• Preferred scenario";
        addChatMessage('ai', response);
    }

    scrollChatToBottom();
}

// Generate campaign based on AI suggestions (Placeholder)
function generateCampaign(campaignType) {
    addChatMessage('user', 'Yes, create it');
    showTypingIndicator();

    setTimeout(function() {
        hideTypingIndicator();

        // Populate form fields with AI-generated data
        $('#name').val('Credential Harvesting - ' + moment().format('YYYY-MM-DD'));

        // Show preview
        showCampaignPreview({
            name: 'Credential Harvesting - ' + moment().format('YYYY-MM-DD'),
            type: 'Credential Harvesting',
            template: 'Password Expiration Notice',
            landingPage: 'Office365 Login',
            targetGroups: 'Sales Department',
            launchDate: moment().add(1, 'day').format('MMMM Do YYYY, h:mm a')
        });

        addChatMessage('ai', 'Great! I\'ve created a campaign preview for you. Review the details and click "Launch Campaign" when ready.');
    }, 2000);
}

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
