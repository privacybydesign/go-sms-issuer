import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  PhoneInput,
  defaultCountries,
  parseCountry,
} from 'react-international-phone';
import 'react-international-phone/style.css';
import { parsePhoneNumberFromString } from 'libphonenumber-js';
import { useNavigate } from "react-router-dom";
import { useAppContext } from '../AppContext';

const isPhoneValid = (input: string) => {
  try {
    const phone = parsePhoneNumberFromString(input);
    return phone?.isValid();
  } catch (error) {
    return false;
  }
};

export default function IndexPage() {
  const { t, i18n } = useTranslation();
  const { phoneNumber, setPhoneNumber} = useAppContext();
  const isValid = isPhoneValid(phoneNumber || '');
  const [showError, setShowError] = useState(false);
  const navigate = useNavigate();

  const countries = defaultCountries.filter((country) => {
    const { iso2 } = parseCountry(country);
    return ['at', 'be', 'bg', 'cy', 'dk', 'de', 'ee', 'fi', 'fr', 'gr', 'hu', 'ie',
      'is', 'it', 'hr', 'lv', 'lt', 'li', 'lu', 'mt', 'mc', 'nl', 'no', 'at',
      'pl', 'pt', 'ro', 'si', 'sk', 'es', 'cz', 'gb', 'se', 'ch'].includes(iso2);
  });

  const onChange = (value: string) => {
    setPhoneNumber(value);
    if (showError && isValid) {
      setShowError(false);
    }
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isValid) {
      setShowError(true);
      return;
    }
    navigate(`/${i18n.language}/validate`);
  };

  return (
    <>
      <form id="container" onSubmit={submit}>
        <header>
          <h1>{t('index_header')}</h1>
        </header>
        <main>
          <div className="sms-form">
            <p>{t('index_explanation')}</p>
            <p>{t('index_multiple_numbers')}</p>
            <label htmlFor="bank-select">{t('phone_number')}</label>
            <PhoneInput
              defaultCountry="nl"
              value={phoneNumber}
              onChange={onChange}
              countries={countries}
            />
            <p>
              {showError && <div className="warning">{t('index_phone_not_valid')}</div>}
            </p>
          </div>
        </main>
        <footer>
          <div className="actions">
            <div></div>
            <button id="submit-button" type="submit">{t('index_start')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
