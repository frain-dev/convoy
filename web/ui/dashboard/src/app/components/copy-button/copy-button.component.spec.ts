import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CopyButtonComponent } from './copy-button.component';
import { RouterTestingModule } from '@angular/router/testing';
import { GeneralService } from 'src/app/services/general/general.service';

describe('CopyButtonComponent', () => {
  let component: CopyButtonComponent;
  let fixture: ComponentFixture<CopyButtonComponent>;
  let notification: jasmine.Spy;
  let originalClipboard: PropertyDescriptor | undefined;
  let writeText: jasmine.Spy;

  beforeEach(async () => {
    notification = jasmine.createSpy('showNotification');
    writeText = jasmine.createSpy('writeText').and.resolveTo();
    originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CopyButtonComponent ],
      providers: [{ provide: GeneralService, useValue: { showNotification: notification } }]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CopyButtonComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  afterEach(() => {
    if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard);
    else delete (navigator as any).clipboard;
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('copies the entire text and announces success only after the clipboard accepts it', async () => {
    const payload = JSON.stringify({ nested: { 'quoted"key': 'x'.repeat(500) } }, null, 4);
    component.textToCopy = payload;
    component.notificationText = 'Payload copied';
    const copied = jasmine.createSpy('copied');
    component.copy.subscribe(copied);
    const event = { stopPropagation: jasmine.createSpy('stopPropagation') };

    await component.copyItem(event);

    expect(event.stopPropagation).toHaveBeenCalled();
    expect(writeText).toHaveBeenCalledOnceWith(payload);
    expect(copied).toHaveBeenCalledTimes(1);
    expect(notification).toHaveBeenCalledOnceWith({ message: 'Payload copied', style: 'info' });
  });

  it('does not announce success when clipboard permission is denied', async () => {
    component.textToCopy = '{"status":"paid"}';
    component.notificationText = 'Payload copied';
    writeText.and.rejectWith(new Error('Clipboard permission denied'));
    const copied = jasmine.createSpy('copied');
    component.copy.subscribe(copied);

    await component.copyItem({ stopPropagation: () => {} });

    expect(writeText).toHaveBeenCalled();
    expect(copied).not.toHaveBeenCalled();
    expect(notification).not.toHaveBeenCalled();
  });

  it('does not write or emit success for empty content', async () => {
    component.textToCopy = '';
    const copied = jasmine.createSpy('copied');
    component.copy.subscribe(copied);
    await component.copyItem({ stopPropagation: () => {} });
    expect(writeText).not.toHaveBeenCalled();
    expect(copied).not.toHaveBeenCalled();
  });
});
