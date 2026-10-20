import { ComponentFixture, TestBed } from '@angular/core/testing';

import { By } from '@angular/platform-browser';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrismComponent } from './prism.component';
import { PrismModule } from './prism.module';

describe('PrismComponent', () => {
  let component: PrismComponent;
  let fixture: ComponentFixture<PrismComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [PrismModule],
      providers: [{ provide: GeneralService, useValue: { showNotification: jasmine.createSpy('showNotification') } }]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(PrismComponent);
    component = fixture.componentInstance;
    component.language = 'json';
  });

  it('offers the full payload for copying even when collapsed', () => {
    const payload = { nested: { 'quoted"key': 'x'.repeat(400) }, nullable: null };
    component.code = JSON.stringify(payload, null, 4);
    fixture.detectChanges();
    const copy = fixture.debugElement.query(By.directive(CopyButtonComponent));
    expect(copy.componentInstance.textToCopy).toBe(component.code);
    expect(JSON.parse(copy.componentInstance.textToCopy)).toEqual(payload);
    expect(component.showPayload).toBeFalse();
    expect(copy.nativeElement.textContent).toContain('Copy');
    expect(fixture.nativeElement.querySelector('code').textContent).toBe(component.code);
  });

  it('updates the copy text when another payload is selected', () => {
    fixture.componentRef.setInput('code', '{"first":1}');
    fixture.detectChanges();
    fixture.componentRef.setInput('code', '{"second":2}');
    fixture.detectChanges();
    const copy = fixture.debugElement.query(By.directive(CopyButtonComponent));
    expect(copy.componentInstance.textToCopy).toBe('{"second":2}');
  });

  it('does not show a copy button without content', () => {
    fixture.detectChanges();
    expect(fixture.debugElement.query(By.directive(CopyButtonComponent))).toBeNull();
  });
});
