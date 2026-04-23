from django.db import models
from django.core.validators import EmailValidator

class WaitlistUser(models.Model):
    class ReferralSource(models.TextChoices):
        TWITTER = 'twitter', 'Twitter/X'
        GITHUB = 'github', 'GitHub'
        FRIEND = 'friend', 'Friend/Colleague'
        BLOG = 'blog', 'Tech Blog'
        OTHER = 'other', 'Other'
    
    email = models.EmailField(
        unique=True,
        validators=[EmailValidator()],
        db_index=True
    )
    name = models.CharField(max_length=100)
    referral_source = models.CharField(
        max_length=20,
        choices=ReferralSource.choices,
        default=ReferralSource.OTHER
    )
    wants_updates = models.BooleanField(default=True)
    position = models.PositiveIntegerField(null=True, blank=True)
    notes = models.TextField(blank=True, null=True)
    
    # Timestamps
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    # Confirmation
    is_confirmed = models.BooleanField(default=False)
    confirmed_at = models.DateTimeField(null=True, blank=True)
    
    # Metadata
    ip_address = models.GenericIPAddressField(null=True, blank=True)
    user_agent = models.TextField(blank=True, null=True)
    
    class Meta:
        ordering = ['position', 'created_at']
        verbose_name = 'Waitlist User'
        verbose_name_plural = 'Waitlist Users'
    
    def __str__(self):
        return f"{self.name} ({self.email})"
    
    def save(self, *args, **kwargs):
        if not self.position:
            # Assign next position
            last_position = WaitlistUser.objects.aggregate(
                models.Max('position')
            )['position__max'] or 0
            self.position = last_position + 1
        super().save(*args, **kwargs)