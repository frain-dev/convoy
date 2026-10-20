import { Location } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrismComponent } from 'src/app/private/components/prism/prism.component';
import { PrismModule } from 'src/app/private/components/prism/prism.module';

// Import only APIs present at the frozen baseline. Grade observable output,
// not proposed helper names or candidate-supplied regression tests.
describe('Issue2886 independent oracle', () => {
  const payload = {
    'quoted"key': 'line\nwith "quotes" and \\slashes',
    nested: { enabled: true, absent: null, items: [1, 'two', false] },
    long: 'synthetic-payload-'.repeat(40)
  };
  const service = new GeneralService({} as ActivatedRoute, {} as Location);

  it('preserves parseable payload JSON', () => {
    const formatted = service.getCodeSnippetString('event_data', payload);
    expect(JSON.parse(formatted)).toEqual(payload);
  });

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [PrismModule],
      providers: [{ provide: GeneralService, useValue: { showNotification: jasmine.createSpy('notification') } }]
    }).compileComponents();
  });

  it('renders all JSON keys and escaped values', () => {
    const fixture = TestBed.createComponent(PrismComponent);
    fixture.componentInstance.language = 'json';
    fixture.componentInstance.code = service.getCodeSnippetString('event_data', payload);
    fixture.detectChanges();
    const code = fixture.nativeElement.querySelector('code');
    expect(code).withContext('payload code element').not.toBeNull();
    expect(JSON.parse(code.textContent)).toEqual(payload);
  });

  it('copies the entire collapsed payload through a real button', async () => {
    const write = spyOn(navigator.clipboard, 'writeText').and.returnValue(Promise.resolve());
    const fixture = TestBed.createComponent(PrismComponent);
    fixture.componentInstance.language = 'json';
    fixture.componentInstance.code = service.getCodeSnippetString('event_data', payload);
    fixture.detectChanges();
    expect(fixture.componentInstance.showPayload).toBeFalse();
    const buttons: HTMLButtonElement[] = Array.from(fixture.nativeElement.querySelectorAll('button'));
    const copy = buttons.find(button => /copy/i.test([
      button.textContent, button.getAttribute('aria-label'), button.getAttribute('title')
    ].join(' ')));
    expect(copy).withContext('accessible copy button').toBeDefined();
    if (!copy) return;
    copy.click();
    await fixture.whenStable();
    expect(write).toHaveBeenCalledTimes(1);
    expect(JSON.parse(write.calls.mostRecent().args[0])).toEqual(payload);
    expect(fixture.componentInstance.showPayload).toBeFalse();
  });
});
