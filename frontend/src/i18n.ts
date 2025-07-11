import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

i18n
    .use(LanguageDetector)
    .use(initReactI18next).init({
        detection: {
            order: ['path', 'navigator'],
            lookupFromPathIndex: 0
        },
        resources: {
            en: {
                translation: {
                    index_title: "Add phone number",
                    index_header: "Add phone number",
                    index_explanation: "Add your mobile phone number in your Yivi app.",
                    index_multiple_numbers: "Do you want to add multiple mobile phone numbers? Then follow these steps for each phone number you want to add.",
                    phone_number: "Phone number",
                    index_start: "Start verification",
                    index_phone_placeholder: "06 12345678",
                    index_phone_not_valid: "The entered phone number is not valid.",
                    validate_bot_control: "The system is validating that you are not a bot. Please wait.",
                    validate_header: "Check your phone number",
                    validate_explanation: "Please check the phone number again for confirmation. Choose 'Back' to correct your phone number.",
                    back: "Back",
                    confirm: "Confirm",
                    error_phone_number_format: 'You did not enter a valid telephone number. Please check whether the telephone number is correct.',
                    error_internal: 'Internal error. Please contact Yivi if this happens more often.',
                    error_sending_sms: 'Sending the SMS message fails. Most likely this is problem in the Yivi system. Please contact Yivi if this happens more often.',
                    error_ratelimit: 'You have requested to many times. Please try again after {{time}}.',
                    error_cannot_validate_token: 'The code cannot be verified. Is there a typo?',
                    error_captcha_failed: 'Captcha verification failed.',
                    error_invalid_captcha: 'Captcha verification failed. Please try again.',
                    error_address_malformed: 'The telephone number you entered is not supported by us. You can only add (European) mobile telephone numbers.',
                    verify: "Verify",
                    receive_sms: "You will receive a text message from Yivi.",
                    steps: "Take the following steps:",
                    step_1: "Open the text message sent by Yivi.",
                    step_2: "Follow the link in the text message.",
                    step_3: "You will be redirected back to your Yivi app.",
                    not_mobile: "Are you not viewing this page on your mobile? Then enter the verification code from the text message below.",
                    verification_code: "Verification code",
                    sending_sms: "SMS message is being sent...",
                    sms_sent: "SMS message has been sent.",
                    verifying_token: "Code is being verified ...",
                    error_header: "Error message",
                    error_default: "An unknown error has occurred. Please try again later.",
                    phone_add_success: "Phone number added.",
                    phone_add_cancel: "Cancelled.",
                    phone_add_error: "Unfortunately, it was not possible to add this phone number to the Yivi app.",
                    done_header: "Phone number added",
                    thank_you: "Thank you for using Yivi, you can close this page now.",
                    again: "Add another number",
                }
            },
            nl: {
                translation: {
                    index_title: "Telefoonnummer toevoegen",
                    index_header: "Telefoonnummer toevoegen",
                    index_explanation: "Zet je mobiele telefoonnummer in je Yivi-app.",
                    index_multiple_numbers: "Wil je meerdere mobiele telefoonnummers toevoegen? Doorloop deze stappen dan voor elk telefoonnummer dat je wilt toevoegen.",
                    phone_number: "Telefoonnummer",
                    index_start: "Start verificatie",
                    index_phone_placeholder: "06 12345678",
                    index_phone_not_valid: "Het ingevoerde telefoonnummer is niet geldig.",
                    validate_bot_control: "Het systeem controleert of je geen bot bent. Even geduld alstublieft.",
                    validate_header: "Telefoonnummer controleren",
                    validate_explanation: "Controleer het telefoonnummer nogmaals ter bevestiging. Kies 'Terug' om je telefoonnummer te corrigeren.",
                    back: "Terug",
                    confirm: "Bevestigen",
                    error_phone_number_format: 'Je hebt geen geldig telefoonnummer ingevoerd. Controleer of het ingevoerde telefoonnummer klopt.',
                    error_internal: 'Interne fout. Neem contact op met Yivi als dit vaker voorkomt.',
                    error_sending_sms: 'De SMS kan niet worden verzonden. Dit is waarschijnlijk een probleem in Yivi. Neem contact op met Yivi als dit vaker voorkomt.',
                    error_ratelimit: 'U heeft te vaak een verzoek gedaan. Probeer het opnieuw na {{time}}.',
                    error_cannot_validate_token: 'De code kon niet worden geverifieerd. Zit er geen typfout in?',
                    error_captcha_failed: 'Captcha verificatie mislukt.',
                    error_invalid_captcha: 'Captcha verificatie mislukt. Probeer het opnieuw.',
                    error_address_malformed: 'Het ingevoerde telefoonnummer wordt niet ondersteund. Je kan alleen mobiele telefoonnummers toevoegen.',
                    verify: "Verifiëren",
                    receive_sms: "Je ontvangt een SMS van Yivi.",
                    steps: "Doorloop de volgende stappen:",
                    step_1: "Open het SMS-bericht afkomstig van Yivi.",
                    step_2: "Kies de link in het SMS-bericht.",
                    step_3: "Je wordt teruggestuurd naar je Yivi-app.",
                    not_mobile: "Bekijk je deze pagina niet op je mobiel? Vul hieronder de verificatiecode uit het SMS-bericht in.",
                    verification_code: "Verificatiecode",
                    sending_sms: "SMS-bericht wordt verstuurd...",
                    sms_sent: "SMS-bericht is verstuurd.",
                    verifying_token: "Code wordt geverifieerd ...",
                    error_header: "Foutmelding",
                    error_default: "Er is een onbekende fout opgetreden. Probeer het later opnieuw.",
                    phone_add_success: "Telefoonnummer toegevoegd.",
                    phone_add_cancel: "Geannuleerd.",
                    phone_add_error: "Het is helaas niet gelukt dit telefoonnummer toe te voegen aan de Yivi-app.",
                    done_header: "Telefoonnummer toegevoegd",
                    thank_you: "Bedankt voor het gebruik van Yivi, u kunt deze pagina nu sluiten.",
                    again: "Nog een telefoonnummer",
                }
            }
        },
        lng: 'nl', // default language
        fallbackLng: 'en',

        interpolation: {
            escapeValue: false, // react already escapes
        }
    });

export default i18n;
