import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from "react-router-dom";
import { useAppContext } from '../AppContext';
import { PhoneInput } from 'react-international-phone';
import { useState, useEffect, useRef } from 'react';
import parsePhoneNumberFromString from 'libphonenumber-js';
import 'altcha';
import 'altcha/i18n/nl';
import 'altcha/i18n/en';

declare global {
  namespace JSX {
    interface IntrinsicElements {
      'altcha-widget': any;
    }
  }
}

export default function ValidatePage() {
  const navigate = useNavigate();
  const [errorMessage, setErrorMessage] = useState<string | undefined>(undefined);
  const { t, i18n } = useTranslation();
  const { phoneNumber } = useAppContext();
  const [captcha, setCaptcha] = useState<string>('');
  const widgetRef = useRef<any>(null);

  useEffect(() => {
    const handleStateChange = (ev: CustomEvent) => {
      if (ev.detail.state === 'verified') {
        setCaptcha(ev.detail.payload);
      }
    };

    const widget = widgetRef.current;
    if (widget) {
      widget.addEventListener('statechange', handleStateChange as EventListener);
      // Set the language based on i18n language
      const language = i18n.language === 'nl' ? 'nl' : 'en';
      widget.setAttribute('language', language);
    }

    return () => {
      if (widget) {
        widget.removeEventListener('statechange', handleStateChange as EventListener);
      }
    };
  }, [i18n.language]);

  const enroll = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    if (!phoneNumber) {
      navigate(`/${i18n.language}/error`);
      return;
    }
    const parsedPhoneNumber = parsePhoneNumberFromString(phoneNumber);

    const response = await fetch(
      '/send',
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          phone: parsedPhoneNumber?.number,
          language: i18n.language,
          captcha: captcha
        })
      }
    );
    // Navigate to the validate page with react router.
    if (response.ok) {
      navigate(`/${i18n.language}/enroll`);
    } else {
      let errorCode = await response.text()
      errorCode = errorCode.trim().replaceAll("-", "_").replaceAll(":", "_").toLowerCase();
      if (errorCode) {
        // If rate limit error, extract the retry time from the response headers
        if (errorCode === 'error_ratelimit') {
          const retryAfter = response.headers.get('Retry-After')
          if (retryAfter) {
            const retryTime = new Date(Date.now() + parseInt(retryAfter) * 1000);
            const formattedTime = retryTime.toLocaleTimeString('nl-NL', { timeZone: 'Europe/Amsterdam' });
            const messageWithTime = t(errorCode, { time: formattedTime })
            setErrorMessage(messageWithTime);
            return;
          }
        }
        // For other errors, just set the error message
        setErrorMessage(t(errorCode));
      } else {
        navigate(`/${i18n.language}/error`);
      }
    }
  }

  return (
    <>
      <form id="container" onSubmit={enroll}>
        <header>
          <h1>{t('validate_header')}</h1>
        </header>
        <main>
          <div className="sms-form">
            {errorMessage && <div id="status-bar" className="alert alert-danger" role="alert">
              <div className="status-container">
                <div id="status">{errorMessage}</div>
              </div>
            </div>
            }
            <p>{t('validate_explanation')}</p>

            <PhoneInput
              defaultCountry="nl"
              value={phoneNumber}
              disabled={true}
            />
            <p>{t('validate_bot_control')}</p>
            <altcha-widget ref={widgetRef} challengeurl="/api/challenge"></altcha-widget>
          </div>
        </main>
        <footer>
          <div className="actions">
            <Link to={`/${i18n.language}`} id="back-button">
              {t('back')}
            </Link>
            <button id="submit-button" disabled={!captcha}>{t('confirm')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
