import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TooltipComponent } from './tooltip.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('TooltipComponent', () => {
  let component: TooltipComponent;
  let fixture: ComponentFixture<TooltipComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, TooltipComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(TooltipComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('keeps the body out of layout as an overlay', () => {
    const body = fixture.nativeElement.querySelector('[data-tooltip-body]') as HTMLElement;
    expect(body.className.split(/\s+/)).toContain('absolute');
  });

  it('lets full-width toggles fill their host', () => {
    component.fillHost = true;
    expect(component.hostClasses.split(/\s+/)).toContain('w-full');
  });

  it('keeps non-interactive tooltips click-through on hover', () => {
    component.interactive = false;
    const classes = component.classes.split(/\s+/);
    expect(classes).toContain('pointer-events-none');
    expect(classes).not.toContain('group-hover:pointer-events-auto');
  });

  it('can stack above dropdown overlays without blocking menus', () => {
    component.stackAboveOverlay = true;
    expect(component.classes.split(/\s+/)).toContain('z-[56]');
  });
});
