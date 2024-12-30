'use strict';

// This var is global, as we need it to verify a phone number.
var phone;

const isInApp = location.href.includes('?inapp=true');

$(function() {
    $('#phone-form').on('submit', onSubmitPhone);
    $('#token-form').on('submit', onSubmitToken);
    setWindow('phone');

    // Is this a link from a SMS message?
    if (document.location.hash.substr(0, '#!verify:'.length) == '#!verify:') {
        verifyTokenFromURL();
    }

    // Use country names translated into their own language
    var countryData = $.fn.intlTelInput.getCountryData();
    $.each(countryData, function(i, country) {
        country.name = country.name.replace(/.+\((.+)\)/, '$1');
    });

    var telInput = $('#phone');
    telInput.change(validateInput);
    telInput.keyup(validateInput);
    telInput.intlTelInput({
        // geoIpLookup: function(callback) {
        //   $.get('http://ipinfo.io', function() {}, 'jsonp').always(function(resp) {
        //     var countryCode = (resp && resp.country) ? resp.country : '';
        //     callback(countryCode);
        //   });
        // },
        initialCountry: 'nl',
        preferredCountries: ['nl'],
        onlyCountries: [
            'at', 'be', 'bg', 'cy', 'dk', 'de', 'ee', 'fi', 'fr', 'gr', 'hu', 'ie',
            'is', 'it', 'hr', 'lv', 'lt', 'li', 'lu', 'mt', 'mc', 'nl', 'no', 'at',
            'pl', 'pt', 'ro', 'si', 'sk', 'es', 'cz', 'gb', 'se', 'ch'
        ],
        utilsScript: 'assets/telwidget/js/utils.js'
    });
    $('#phone-confirm').intlTelInput({
        initialCountry: 'nl',
        allowDropdown: false,
        utilsScript: 'assets/telwidget/js/utils.js'
    });
});

function setWindow(window, back) {
    $('[id^=block-]').hide();
    $('#block-'+window).show();
    $('#submit-button').text(MESSAGES['button-' + window]);

    // Put h1 in header when not being on mobile
    const h1 = $('#block-'+window + ' h1');
    if (isInApp) {
        $('header').hide();
        h1.show();
    } else {
        $('header').html(h1.clone().show()).show();
        h1.hide();
    }

    const backButton = $('#back-button');
    backButton.off();
    if (back) {
        backButton
          .click(() => {clearStatus(); setWindow(back); return false;})
          .removeClass('button-hidden');
    } else if (history.length >  1) {
        backButton
          .click(() => {clearStatus(); history.back(); return false;})
          .removeClass('button-hidden');
    } else {
        backButton.addClass('button-hidden');
    }

    const submitButton = $('#submit-button');
    submitButton.off();
    switch (window) {
        case 'phone':
        case 'confirm':
            submitButton.click(() => $('#submit-phone').click());
            break;
        case 'token':
            submitButton.click(() => $('#submit-token').click());
            break;
    }
}

function validateInput() {
    var telInput = $('#phone');
    telInput.removeClass(['error', 'invalid', 'not-mobile']);
    if ($.trim(telInput.val())) {
        if (telInput.intlTelInput('isValidNumber')) {
            var numberType = telInput.intlTelInput('getNumberType');
            var numberTypes = intlTelInputUtils.numberType;
            // In some countries there is no distinction between fixed line and mobile.
            // Therefore check for FIXED_LINE_OR_MOBILE too.
            if ([numberTypes.MOBILE, numberTypes.FIXED_LINE_OR_MOBILE].includes(numberType)) {
                clearStatus();
            } else {
                telInput.addClass('invalid not-mobile');
            }
        } else {
            telInput.addClass('invalid');
        }
    }
}

function verifyTokenFromURL() {
    // format: #!verify:+3112345678:ABCDEF
    var parts = document.location.hash.split(':');
    if (parts.length != 3) {
        return;
    }

    if ('history' in window) {
        // remove hash
        history.replaceState(null, '', document.location.pathname);
    }

    phone = parts[1];
    var token = parts[2];

    // Now do the same thing as when a user manually submitted the form
    // Enter the phone number
    $('#phone-form input[type=tel]').val(phone);
    $('#phone-form input').prop('disabled', true);

    // Get a token and fill it here
    $('#token-form input[type=text]').val(token);
    $('#block-token').show();

    // And submit!
    $('#token-form').submit();
}

function onSubmitPhone(e) {
    e.preventDefault();

    var phoneInput = $('#phone');

    if (phoneInput.hasClass('invalid')) {
        phoneInput.addClass('error');
        setStatus('warning',
          phoneInput.hasClass('not-mobile') ? MESSAGES['error:address-malformed'] : MESSAGES['error:phone-number-format']);
        return;
    }

    phone = phoneInput.intlTelInput("getNumber");
    var country = phoneInput.intlTelInput("getSelectedCountryData");

    if ($('#block-confirm').is(':hidden')) {
        $('#phone-confirm').intlTelInput("setNumber", phone);
        $('#phone-confirm').intlTelInput("setCountry", country.iso2);
        setWindow('confirm', 'phone');
        return;
    }

    setStatus('info', MESSAGES['sending-sms']);
    $.ajax({
        url: CONF.API_ENDPOINT + 'send', 
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({phone: phone, language: MESSAGES['lang']})
    })
        .done(function(e) {
            console.log('sent SMS:', e);
            setStatus('success', MESSAGES['sms-sent']);
            setWindow('token', 'phone');
        })
        .fail(function(e) {
            setWindow('phone');
            var errormsg = e.responseText;
            console.error('failed to submit phone number:', errormsg);
            if (!errormsg || !MESSAGES[errormsg]) {
                errormsg = 'error:internal';
            }
            if (errormsg == 'error:ratelimit') {
                var retryAfter = e.getResponseHeader('Retry-After');
                // In JavaScript, we can mostly ignore the fact we're dealing
                // with a string here and treat it as an integer...
                if (retryAfter < 60) {
                    var timemsg = MESSAGES['seconds'];
                    if (retryAfter == 1) {
                        var timemsg = MESSAGES['second'];
                    }
                } else if (retryAfter < 60*60) {
                    retryAfter = Math.round(retryAfter / 60);
                    var timemsg = MESSAGES['minutes'];
                    if (retryAfter == 1) {
                        var timemsg = MESSAGES['minute'];
                    }
                } else {
                    retryAfter = Math.round(retryAfter / 60 / 60);
                    var timemsg = MESSAGES['hours'];
                    if (retryAfter == 1) {
                        var timemsg = MESSAGES['hour'];
                    }
                }
                setStatus('danger', MESSAGES[errormsg].replace('%time%',
                    timemsg.replace('%n%', retryAfter)));
            } else {
                setStatus('danger', MESSAGES[errormsg]);
            }
        });
}

function onSubmitToken(e) {
    e.preventDefault();
    var token = $('#token-form input[type=text]').val().trim().toUpperCase();
    setStatus('info', MESSAGES['verifying-token']);
    $.ajax({
        url: CONF.API_ENDPOINT + 'verify', 
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({phone: phone, token: token})
    })
        .done(function(jwt) {
            console.log('received JWT:', jwt);
            setStatus('info', MESSAGES['issuing-credential']);
            yivi.newPopup({
                language: MESSAGES['lang'],
                session: {
                    url: CONF.IRMASERVER,
                    start: {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'text/plain',
                        },
                        body: jwt,
                    },
                    result: false,
                },
            })
                .start()
                .then(() => {
                    setStatus('success', MESSAGES['phone-add-success']);
                })
                .catch((e) => {
                    if (e === 'Aborted') {
                        console.warn('cancelled:', e);
                        setStatus('info', MESSAGES['phone-add-cancel']);
                    } else {
                        setStatus('danger', MESSAGES['phone-add-error']);
                        console.error('error:', e);
                    }
                });
        })
        .fail(function(e) {
            var errormsg = e.responseText;
            console.error('failed to submit phone number:', errormsg);
            if (!errormsg || errormsg.substr(0, 6) != 'error:') {
                errormsg = 'error:internal';
            }
            setStatus('danger', MESSAGES[errormsg]);
        });
}

function clearStatus() {
    $('#status-bar').addClass('hidden');
}

function setStatus(alertType, message) {
    $('#status').html(message);
    $('#status-bar')
        .removeClass('alert-success')
        .removeClass('alert-info')
        .removeClass('alert-warning')
        .removeClass('alert-danger')
        .addClass('alert-'+alertType)
        .removeClass('hidden');
    window.scrollTo(0,0);
}
