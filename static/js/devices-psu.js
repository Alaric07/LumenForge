(function(root,factory){
    const api=factory();
    if(typeof module==="object"&&module.exports){module.exports=api;}
    if(root&&root.document){const init=function(){api.init(root);};if(root.document.readyState==="loading"){root.document.addEventListener("DOMContentLoaded",init);}else{init();}}
})(typeof window==="undefined"?null:window,function(){
    function init(browser){
        const workspace=browser.document.querySelector("[data-lf-psu-workspace]");
        if(!workspace){return;}
        const mode=workspace.querySelector("[data-lf-psu-fan-mode]"),status=workspace.querySelector("[data-lf-psu-status]");
        if(!mode){return;}
        let confirmed=mode.value;
        let requestActive=false;
        let pendingValue="";
        mode.addEventListener("change",function(){
            if(requestActive){mode.value=pendingValue;return Promise.resolve();}
            const submittedValue=mode.value,value=Number(submittedValue);
            if(!Number.isInteger(value)||[0,4,5,6,7,8,9,10].indexOf(value)<0){mode.value=confirmed;return Promise.resolve();}
            requestActive=true;pendingValue=submittedValue;mode.disabled=true;
            return browser.fetch("/api/psu/speed",{method:"POST",body:JSON.stringify({deviceId:workspace.dataset.lfDeviceId,fanMode:value})}).then(async function(response){
                const result=response.ok?await response.json():null;
                if(!result||result.status!==1){throw new Error("fan mode rejected");}
                confirmed=submittedValue;
                if(browser.LumenForgeDevicesToast){browser.LumenForgeDevicesToast("✓ Saved","success",1500);}
            }).catch(function(){
                mode.value=confirmed;
                if(status){status.textContent="Couldn’t save PSU fan mode.";}
            }).finally(function(){requestActive=false;pendingValue="";mode.disabled=false;});
        });
    }
    return{init:init};
});
