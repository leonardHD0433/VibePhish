var accounts = []
var emailTypes = []

// Load email types from API and populate dropdown
function loadEmailTypes() {
    api.email_types.get()
        .success(function (types) {
            emailTypes = types
            var $typeSelect = $("#type")
            $typeSelect.empty()
            $typeSelect.append('<option value="">-- Select Type --</option>')
            $.each(types, function (i, type) {
                $typeSelect.append('<option value="' + escapeHtml(type.value) + '">' + escapeHtml(type.display_name) + '</option>')
            })
        })
        .error(function () {
            errorFlash("Error loading email types")
        })
}

// Save attempts to POST to /api/email_accounts/
function save(idx) {
    var account = {}
    account.email = $("#email").val()
    account.email_type = $("#type").val()
    account.is_active = $("#is_active").prop("checked")

    if (idx != -1) {
        account.id = accounts[idx].id
        api.email_accounts.put(account)
            .success(function (data) {
                successFlash("Email account updated successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.email_accounts.post(account)
            .success(function (data) {
                successFlash("Email account added successfully! n8n OAuth2 credential created: " + data.n8n_credential_name + ". Configure OAuth2 in n8n UI.")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#email").val("")
    $("#type").val("")
    $("#is_active").prop("checked", true)
    $("#modal").modal('hide')
}

function edit(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    if (idx == -1) {
        $("#modalLabel").text("New Sending Profile")
        dismiss()
        $("#modal").modal('show')
    } else {
        $("#modalLabel").text("Edit Sending Profile")
        var account = accounts[idx]
        $("#email").val(account.email)
        $("#type").val(account.email_type)
        $("#is_active").prop("checked", account.is_active)
        $("#modal").modal('show')
    }
}

function deleteAccount(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the sending profile. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete Profile",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.email_accounts.delete(accounts[idx].id)
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
                'Sending Profile Deleted!',
                'This sending profile has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function load() {
    $("#accountsTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.email_accounts.get()
        .success(function (response) {
            accounts = response
            $("#loading").hide()
            if (accounts.length > 0) {
                $("#accountsTable").show()
                accountsTable = $("#accountsTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                accountsTable.clear()
                $.each(accounts, function (i, account) {
                    var statusBadge = account.is_active
                        ? '<span class="label label-success">Active</span>'
                        : '<span class="label label-default">Inactive</span>'

                    accountsTable.row.add([
                        escapeHtml(account.email),
                        escapeHtml(account.email_type),
                        escapeHtml(account.n8n_credential_name || 'N/A'),
                        escapeHtml(account.usage_count || 0),
                        statusBadge,
                        "<div class='pull-right'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Account' onclick='edit(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Account' onclick='deleteAccount(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]).draw()
                })
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching sending profiles")
        })
}

$(document).ready(function () {
    // Initialize tooltips
    $('[data-toggle="tooltip"]').tooltip()
    // Load email types for dropdown
    loadEmailTypes()
    // Load email accounts
    load()
})
