"""Authored synthetic histories; generation is deterministic and makes no model calls."""
import json
from pathlib import Path
from datetime import datetime, timedelta, timezone

ROOT = Path(__file__).parent
PERSONAS = [
    dict(id="nadia", split="development", theme="accessibility research trip", place1="Kyoto", org1="Cedar Lab", place2="Sendai", org2="Maple Mobility", old="2026-10-12", new="2026-10-14", proposal="Osaka", format="PDF", style="light background", language="English", reason="the host laboratory moved its accessibility workshop", owner="Inez"),
    dict(id="marco", split="development", theme="community energy project", place1="Bristol", org1="Harbor Energy", place2="Bath", org2="Willow Housing", old="2026-11-03", new="2026-11-06", proposal="Cardiff", format="Markdown", style="plain tables", language="Italian", reason="the site engineer is unavailable on the original date", owner="Dev"),
    dict(id="leena", split="heldout", theme="museum conservation study", place1="Turku", org1="Birch Museum", place2="Tampere", org2="Amber Archive", old="2027-02-09", new="2027-02-11", proposal="Oulu", format="DOCX", style="high contrast", language="Finnish", reason="the archive moved its conservation demonstration", owner="Nora"),
    dict(id="owen", split="heldout", theme="volunteer training tour", place1="Galway", org1="Reed Rescue", place2="Cork", org2="Beacon Volunteers", old="2027-03-17", new="2027-03-20", proposal="Limerick", format="HTML", style="large text", language="Irish", reason="the venue is closed for maintenance on the original date", owner="Asha"),
]


def build(p):
    pid=p['id']; events=[]; seq=0
    # Each slot is a separate conversation on a different day; no derived summaries are authored here.
    sessions = [
        [('user',f"I am organizing my {p['theme']}. For materials you prepare for me, use {p['format']}, {p['style']}, and {p['language']}. Treat these as my default preferences."),
         ('assistant',"I will retain your presentation preferences for future work."),
         ('user',"Unrelated: I enjoyed a documentary about ocean currents yesterday. That does not change this project's scope.")],
        [('user',f"Confirmed first visit: I will meet {p['org1']} in {p['place1']} on {p['old']}. This is an approved appointment."),
         ('assistant',f"A possible addition would be a visit to {p['proposal']}. This is only my suggestion, not a booking or your decision."),
         ('user',"Please do not book any extra cities. Keep suggestions separate from confirmed appointments.")],
        [('user',f"A second confirmed organization is {p['org2']} in {p['place2']}. We have not agreed its date yet."),
         ('user',f"{p['owner']} owns coordinating the first visit; I will coordinate the second visit myself."),
         ('assistant',"The travel plan now has two confirmed organizations. Dates and owners remain separate facts.")],
        [('user',f"Correction to the first visit: {p['org1']} in {p['place1']} is now on {p['new']}, replacing {p['old']}. Reason: {p['reason']}. The organization and coordinator have not changed."),
         ('assistant',f"Historical recap only: the first visit used to be on {p['old']}. That is not the current date."),
         ('user',"Unrelated: compare two ways to organize a bookshelf. This is not a request to change travel preferences.")],
        [('user',f"For the briefing, show the confirmed organizations, the current first-visit date, and its coordinator. Use my existing document preferences."),
         ('assistant',f"I could switch the briefing to PNG and add {p['proposal']}; these are unapproved suggestions."),
         ('user',f"No, keep the document preferences I originally gave. I have not approved the {p['proposal']} addition.")],
        [('user',"No hotel has been selected or booked. Do not infer one from the meeting location."),
         ('user',"Prepare future answers from my confirmed plan and explicitly distinguish old dates and unapproved suggestions."),
         ('assistant',"I understand: maintain current decisions and say when details have not been established.")],
    ]
    for session, turns in enumerate(sessions,1):
        for role,text in turns:
            seq+=1
            events.append({'eventId':f'{pid}-{seq:03d}','sessionId':f'{pid}-session-{session}', 'occurredAt':(datetime(2026,9,1,12,tzinfo=timezone.utc)+timedelta(days=session-1,seconds=seq)).isoformat().replace('+00:00','Z'),'sequence':str(seq),'actor':{'role':role,'agentId':'synthetic-history-author'},'source':{'id':f'persona-{pid}-v1','format':'authored-synthetic','version':'1'},'message':{'text':text}})
    def eid(n): return f'{pid}-{n:03d}'
    specs=[
        ('preferences','preferences','What format, visual style, and language should you use for my next briefing?',{'format':p['format'],'style':p['style'],'language':p['language']},{'format':[eid(1)],'style':[eid(1)],'language':[eid(1)]}),
        ('current-date','currentness',f'When is my meeting with {p["org1"]} now?',{'date':p['new']},{'date':[eid(10)]}),
        ('date-change','temporal',f'How did the date for {p["org1"]} change, and why?',{'previous_date':p['old'],'current_date':p['new'],'reason':p['reason']},{'previous_date':[eid(4),eid(10)],'current_date':[eid(10)],'reason':[eid(10)]}),
        ('organizations','cross_session','Which organizations am I confirmed to meet, and in which cities?',{'first_organization':p['org1'],'first_city':p['place1'],'second_organization':p['org2'],'second_city':p['place2']},{'first_organization':[eid(4),eid(10)],'first_city':[eid(4),eid(10)],'second_organization':[eid(7)],'second_city':[eid(7)]}),
        ('proposal-status','authority',f'Is the proposed {p["proposal"]} visit confirmed, and who introduced the suggestion?',{'confirmed':False,'suggested_by':'assistant'},{'confirmed':[eid(6),eid(15)],'suggested_by':[eid(5),eid(14)]}),
        ('briefing','cross_session','Prepare the key details for my first-visit briefing using my preferences.',{'organization':p['org1'],'date':p['new'],'coordinator':p['owner'],'format':p['format']},{'organization':[eid(4),eid(10)],'date':[eid(10)],'coordinator':[eid(8)],'format':[eid(1)]}),
        ('hotel','unknown','Which hotel have I booked for the first visit?',{'hotel':None,'booked':False},{'hotel':[eid(16)],'booked':[eid(16)]}),
        ('coordinator','single_fact',f'Who is coordinating my visit to {p["org1"]}?',{'coordinator':p['owner']},{'coordinator':[eid(8)]}),
    ]
    cases=[]
    for name,family,question,answer,sources in specs:
        fields={key:('boolean' if isinstance(value,bool) else 'string or null' if value is None else 'string') for key,value in answer.items()}
        prompt=question+'\nReturn the answer as a JSON object with these fields/types: '+json.dumps(fields)+'. Use ISO dates. Include citations mapping every answer field to original event IDs. For a stated reason, copy the reason clause from the original evidence verbatim. Use null when the answer is unknown.'
        cases.append({'id':f'{pid}-{name}','history_id':pid,'split':p['split'],'family':family,'input':prompt,'expected':answer,'gold_sources':sources})
    return events,cases


def main():
    cases=[]
    for p in PERSONAS:
        events,questions=build(p); cases.extend(questions)
        folder=ROOT/p['id'];folder.mkdir(exist_ok=True)
        (folder/'history.jsonl').write_text(''.join(json.dumps(e,ensure_ascii=False)+'\n' for e in events))
    (ROOT/'cases.json').write_text(json.dumps({'version':1,'synthetic':True,'cases':cases},indent=2,ensure_ascii=False)+'\n')

if __name__=='__main__':main()
