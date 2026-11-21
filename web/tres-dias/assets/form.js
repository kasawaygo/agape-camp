document.querySelectorAll('[data-form]').forEach(form=>{
 const message=form.querySelector('.form-message');
 const checks=[...form.querySelectorAll('input[name="departments"]')];
 const allergyRadios=[...form.querySelectorAll('input[name="hasAllergies"]')];
 const details=form.querySelector('[name="allergyDetails"]');
 const syncAllergy=()=>{const yes=form.querySelector('[name="hasAllergies"]:checked')?.value==='true';details.disabled=!yes;details.required=yes;if(!yes)details.value=''};
 allergyRadios.forEach(x=>x.addEventListener('change',syncAllergy));syncAllergy();
 form.addEventListener('submit',async event=>{
  event.preventDefault();message.className='form-message';message.textContent='';
  const selected=checks.filter(x=>x.checked);const warning=form.querySelector('.inline-error');
  if(checks.length&&(selected.length<2||selected.length>3)){warning.hidden=false;checks[0].focus();return}if(warning)warning.hidden=true;
  const data=new FormData(form);const payload={formType:form.dataset.form,name:data.get('name'),birthDate:data.get('birthDate')||'',city:data.get('city'),phone:data.get('phone'),church:data.get('church'),attendedBefore:data.has('attendedBefore')?data.get('attendedBefore')==='true':null,clothingSize:data.get('clothingSize'),hasAllergies:data.get('hasAllergies')==='true',allergyDetails:data.get('allergyDetails')||'',allergyConsent:data.has('allergyConsent'),departments:selected.map(x=>x.value),previousDepartments:data.get('previousDepartments')||'',readRoe:data.get('readRoe')||''};
  const button=form.querySelector('button[type="submit"]');button.disabled=true;button.textContent='Отправляем…';
  try{const response=await fetch('/api/tres-dias/register',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});const result=await response.json();if(!response.ok)throw new Error(result.message||'Не получилось отправить анкету.');form.reset();syncAllergy();message.textContent=result.message;message.classList.add('success')}
  catch(error){message.textContent=error.message;message.classList.add('error')}
  finally{button.disabled=false;button.textContent='Отправить анкету'}
 });
});
