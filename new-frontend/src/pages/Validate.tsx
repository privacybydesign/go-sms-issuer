import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from "react-router-dom";
import { useAppContext } from '../AppContext';
import { PhoneInput } from 'react-international-phone';
import { useState } from 'react';

export default function ValidatePage() {
  const navigate = useNavigate();
  const { error, setError } = useState<string | null>(null);
  const { t, i18n } = useTranslation();
  const { phoneNumber } = useAppContext();

  const enroll = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const response = await fetch(
      '/send',
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          phone: phoneNumber,
          language: i18n.language,
        })
      }
    );
    // Navigate to the validate page with react router.
    if (response.ok) {
      navigate(`/${i18n.language}/enroll`);
    } else {
      switch(await response.text()){ 
        case "error:ratelimit":
          setError(t('validate_error_ratelimit'));
          break;
        default:
          navigate(`/${i18n.language}/error`);
      }
    }
  };

  return (
    <>
      <form id="container" onSubmit={enroll}>
        <header>
          <h1>{t('validate_header')}</h1>
        </header>
        <main>
          <div className="sms-form">

            <p>{t('validate_explanation')}</p>

            <PhoneInput
              defaultCountry="nl"
              value={phoneNumber}
              disabled={true}
            />
          </div>
        </main>
        <footer>
          <div className="actions">
            <Link to="/" id="back-button">
              {t('back')}
            </Link>
            <button id="submit-button" >{t('confirm')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
