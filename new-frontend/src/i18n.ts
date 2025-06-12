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
                    index_phone_not_valid: "Phone number is not valid",
                    validate_header: "Check your phone number",
                    validate_explanation: "Please check the phone number again for confirmation. Choose 'Back' to correct your phone number.",
                    back: "Back",
                    confirm: "Confirm",
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
                    index_phone_not_valid: "Telefoonnummer is niet geldig",
                    validate_header: "Telefoonnummer controleren",
                    validate_explanation: "Controleer het telefoonnummer nogmaals ter bevestiging. Kies 'Terug' om je telefoonnummer te corrigeren.",
                    back: "Terug",
                    confirm: "Bevestigen",
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
