import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TokenModalComponent } from './token-modal.component';

describe('TokenModalComponent', () => {
  let component: TokenModalComponent;
  let fixture: ComponentFixture<TokenModalComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ TokenModalComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(TokenModalComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
