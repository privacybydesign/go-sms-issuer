import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  PhoneInput,
  defaultCountries,
  parseCountry,
} from 'react-international-phone';
import 'react-international-phone/style.css';
import { PhoneNumberUtil } from 'google-libphonenumber';
import { useNavigate } from "react-router-dom";

const phoneUtil = PhoneNumberUtil.getInstance();

const isPhoneValid = (phone: string) => {
  try {
    return phoneUtil.isValidNumber(phoneUtil.parseAndKeepRawInput(phone));
  } catch (error) {
    return false;
  }
};

export default function IndexPage() {
  const { t, i18n } = useTranslation();
  const [phone, setPhone] = useState('');
  const isValid = isPhoneValid(phone);
  const navigate = useNavigate();

  const countries = defaultCountries.filter((country) => {
    const { iso2 } = parseCountry(country);
    return ['at', 'be', 'bg', 'cy', 'dk', 'de', 'ee', 'fi', 'fr', 'gr', 'hu', 'ie',
      'is', 'it', 'hr', 'lv', 'lt', 'li', 'lu', 'mt', 'mc', 'nl', 'no', 'at',
      'pl', 'pt', 'ro', 'si', 'sk', 'es', 'cz', 'gb', 'se', 'ch'].includes(iso2);
  });


  const submit = async (e: React.FormEvent) => {
    e.preventDefault();

    const response = await fetch(
      '/send',
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          phone: phone,
          language: i18n.language,
        })
      }
    );
    // Navigate to the enroll page with react router.
    if (response.ok) {
      navigate("/validate");
    } else {

    }
  }

  return (
    <>
      <form id="container" onSubmit={submit}>
        <header>
          <h1>{t('index_header')}</h1>
        </header>
        <main>
          <div id="idin-form">
            <p>{t('index_explanation')}</p>
            <p>{t('index_multiple_numbers')}</p>
            <label htmlFor="bank-select">{t('phone_number')}</label>
            {/* Phone input */}
            <PhoneInput
              defaultCountry="nl"
              value={phone}
              onChange={setPhone}
              countries={countries}
            />
            <p>
              {!isValid && <div className="warning">{t('index_phone_not_valid')}</div>}
            </p>
          </div>
        </main>
        <footer>
          <div className="actions">
            <div></div>
            <button id="submit-button" disabled={!isValid} type="submit">{t('index_start')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
